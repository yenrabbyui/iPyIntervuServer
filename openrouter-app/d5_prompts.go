package main

import (
	_ "embed"
	"fmt"
	"strings"
)

// Bug Hunting level descriptions are read from the reviewed drafts until they move into
// the weekly rubrics (design §8.1).
//
//go:embed D5-bug-rubric-drafts.md
var bugRubricDrafts string

const d5OpeningMaxTokens = 300
const d5BugOpeningMaxTokens = 400
const d5ReplyMaxTokens = 250
const d5CoachingMaxTokens = 500

// markdownSection returns the "## " section whose title starts with titlePrefix, up to
// the next "## " heading, or "".
func markdownSection(md, titlePrefix string) string {
	lines := strings.Split(md, "\n")
	var out []string
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			in = strings.HasPrefix(strings.TrimPrefix(line, "## "), titlePrefix)
		}
		if in {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// withoutSections drops the "## " sections whose titles start with any of the prefixes.
func withoutSections(md string, prefixes ...string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			title := strings.TrimPrefix(line, "## ")
			skip = false
			for _, p := range prefixes {
				if strings.HasPrefix(title, p) {
					skip = true
				}
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func readInstructionFile(path string) string {
	data, err := instructionFS.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// d5RubricForMode returns the level descriptions the Evaluator judges a mode against.
// Grading rules ("Overall Guidance", rating-label rules) are left out: Go applies those.
func d5RubricForMode(week int, mode string) string {
	if mode == modeBug {
		shared := markdownSection(bugRubricDrafts, "Shared descriptions")
		weekly := markdownSection(bugRubricDrafts, fmt.Sprintf("Week %d ", week))
		return strings.TrimSpace(shared + "\n\n" + weekly)
	}
	rubric := readInstructionFile(fmt.Sprintf("env/rubrics/week%d_rubric.md", week))
	return withoutSections(rubric, "Overall Guidance", "Allowed rating labels")
}

// bugSuitableDefects returns the week's "Suitable defects" note from the Bug drafts.
func bugSuitableDefects(week int) string {
	section := markdownSection(bugRubricDrafts, fmt.Sprintf("Week %d ", week))
	start := strings.Index(section, "**Suitable defects**")
	if start < 0 {
		return ""
	}
	rest := section[start:]
	if end := strings.Index(rest, "\n**What strategy"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(strings.ReplaceAll(rest, "**", ""))
}

// weekFocus lists the selected week's topics from its key-concepts file, for the opening
// focus line (design §4, variants).
func weekFocus(state *AgentSessionState) string {
	text := readInstructionFile(fmt.Sprintf("env/IPYIntervu_support_files/week%d_key_concepts.md", state.CurrentWeekNumber))
	section := markdownSection(text, "Topics Covered")
	var topics []string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			topics = append(topics, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- ")))
		}
	}
	if len(topics) == 0 {
		return state.SelectedKeyConcept
	}
	return strings.Join(topics, ", ")
}

func weekGuide(week int) string {
	return strings.TrimSpace(readInstructionFile(fmt.Sprintf("env/IPYIntervu_support_files/week%d_key_concepts.md", week)) +
		"\n\n" + readInstructionFile(fmt.Sprintf("env/IPYIntervu_support_files/week%d_competency_guide.md", week)))
}

func allowedPythonIdeas(week int) string {
	var ideas []string
	for w := 2; w <= week && w <= lastSyllabusWeek; w++ {
		ideas = append(ideas, conceptsIntroducedByWeek[w]...)
	}
	return strings.Join(ideas, ", ")
}

func loopBanApplies(week int) bool {
	return week >= 7
}

const d5LoopNote = " Never ask for or suggest `while True`, `break` or `continue`; if the candidate uses them, ask why they chose that."

func modeLabel(mode string) string {
	switch mode {
	case modeConceptual:
		return "conceptual"
	case modeCode:
		return "coding"
	case modeBug:
		return "debugging"
	default:
		return "interview"
	}
}

type d5MoveKind string

const (
	moveOpenFirst       d5MoveKind = "OPEN_FIRST"
	moveOpenMode        d5MoveKind = "OPEN_MODE"
	moveFollowUp        d5MoveKind = "FOLLOW_UP"
	moveClarify         d5MoveKind = "CLARIFY"
	moveRedirectVague   d5MoveKind = "REDIRECT_VAGUE"
	moveRequestCode     d5MoveKind = "REQUEST_CODE"
	moveCodeFollowUp    d5MoveKind = "CODE_FOLLOW_UP"
	moveCloseMode       d5MoveKind = "CLOSE_MODE"
	moveCoaching        d5MoveKind = "COACHING"
	moveCoachingLater   d5MoveKind = "COACHING_DEFERRED"
	moveResultsReminder d5MoveKind = "RESULTS_REMINDER"
)

// d5Move is the director's decision for one student message.
type d5Move struct {
	Kind        d5MoveKind
	Instruction string
	// Target is the rubric dimension the question in this reply targets.
	Target    string
	MaxTokens int
	Reason    string
}

func (m d5Move) isOpening() bool {
	return m.Kind == moveOpenFirst || m.Kind == moveOpenMode
}

// openingInstruction is the move text for a mode's first reply. needsCompany asks the
// model for the COMPANY line (first opening only).
func openingInstruction(state *AgentSessionState, mode string, needsCompany bool) string {
	var b strings.Builder
	switch mode {
	case modeConceptual:
		if needsCompany {
			fmt.Fprintf(&b, "Start the interview. Your reply must begin with one line in exactly this form:\nCOMPANY: <company name> | <a few words on what the company does>\nChoose a realistic company where someone with a %s background might work. ", state.StudentMajor)
		} else {
			b.WriteString("Start the interview. ")
		}
		b.WriteString("Do not introduce yourself; that has been done. ")
		if state.isProblemDecompositionWeek() {
			b.WriteString("Describe one realistic work problem at the company (2–4 sentences) that someone would need to break into inputs, steps and outputs, then ask how they would start breaking it down. Do not break it down yourself and do not mention code.")
		} else {
			fmt.Fprintf(&b, "Describe one realistic work situation at the company (2–4 sentences) that connects to %s, then ask one conceptual question about it. No code.", weekFocus(state))
		}
	case modeCode:
		fmt.Fprintf(&b, "Start the coding part of the interview. Do not introduce yourself; that has been done. Present one small programming task from work at the company, framed only as the data available and what is wanted, using exactly these two labels:\nData available: <what information the program gets>\nWhat's wanted: <what the program should produce>\nThe task should exercise %s and be solvable with only these Python ideas: %s. Do not list steps, hints or a solution. Then ask one question: how they would break the problem down before writing any code.",
			weekFocus(state), allowedPythonIdeas(state.CurrentWeekNumber))
	case modeBug:
		fmt.Fprintf(&b, "Start the debugging part of the interview. Do not introduce yourself; that has been done. First say in one sentence what a small internal tool at the company is supposed to do. Then show a short Python snippet (5–12 lines) in a ```python block with exactly one defect related to %s. Use only these Python ideas: %s. Do not point out, hint at, or comment on the defect. Then ask one question: how they would go about finding what's wrong.",
			weekFocus(state), allowedPythonIdeas(state.CurrentWeekNumber))
		if defects := bugSuitableDefects(state.CurrentWeekNumber); defects != "" {
			b.WriteString("\nIdeas for the defect (pick one and adapt it to the company):\n" + defects)
		}
	}
	b.WriteString("\nFor this opening reply only, you may use up to 150 words plus any code block.")
	return b.String()
}

// d5InterviewerSystemPrompt builds the Interviewer prompt (design §4): the static persona
// block first so prefix caching can hit, then the dynamic notes and the move.
func d5InterviewerSystemPrompt(state *AgentSessionState, sess *d5Session, mode string, move d5Move, brief *d5Brief) string {
	p := sess.Personas[mode]
	var b strings.Builder
	if sess.CompanyName != "" {
		fmt.Fprintf(&b, "You are %s, %s at %s", p.Name, withArticle(p.Role), sess.CompanyName)
		if sess.CompanyDomain != "" {
			fmt.Fprintf(&b, " (%s)", sess.CompanyDomain)
		}
		b.WriteString(".\n")
	} else {
		fmt.Fprintf(&b, "You are %s, %s at a company in the candidate's field; you will name the company.\n", p.Name, withArticle(p.Role))
	}
	week1 := state.isProblemDecompositionWeek()
	if week1 {
		fmt.Fprintf(&b, "You are interviewing a candidate with a %s background for an entry-level role. This conversation is about breaking real-world problems into inputs, steps and outputs; never discuss code or Python.\n", state.StudentMajor)
	} else {
		fmt.Fprintf(&b, "You are interviewing a candidate with a %s background for an entry-level role that uses Python.\n", state.StudentMajor)
	}
	b.WriteString(`
How you speak:
- A warm, professional interviewer and a colleague at the company, not a teacher.
  Never mention weeks, courses, classes, homework, or what the candidate has studied.
- Respond naturally to what the candidate just said in one or two sentences, then ask
  exactly ONE question. Then stop.
- Never say whether an answer was right, explain, hint, give sample answers, or answer
  your own question. Never write the candidate's reply.
- Use only details the candidate actually gave. Don't introduce yourself again,
  restate the scenario, or say you are moving on or wrapping up.
`)
	if !week1 {
		fmt.Fprintf(&b, "- Use only these Python ideas: %s. Never mention any other Python\n  feature, even to rule it out. Never mention dictionaries.", allowedPythonIdeas(state.CurrentWeekNumber))
		if loopBanApplies(state.CurrentWeekNumber) {
			b.WriteString(d5LoopNote)
		}
		b.WriteString("\n")
	}
	b.WriteString("- Plain text, under 90 words.\n\n--- (dynamic below this line) ---\n")

	if material := sess.Material[mode]; material != "" {
		b.WriteString("Scenario: " + material + "\n")
	}
	if !move.isOpening() {
		var notes []string
		if brief != nil && brief.Mode == mode {
			if brief.Next != "" {
				notes = append(notes, brief.Next)
			}
			if len(brief.Issues) > 0 {
				notes = append(notes, "Correct this in your reply: "+strings.Join(brief.Issues, "; ")+".")
			}
		}
		avoid := append([]string(nil), state.ModeQuestionsAsked...)
		if brief != nil && brief.Mode == mode {
			avoid = append(avoid, brief.Avoid...)
		}
		if len(avoid) > 0 {
			notes = append(notes, "Do not re-ask: "+strings.Join(avoid, " | "))
		}
		if len(notes) > 0 {
			b.WriteString("Colleague's private notes: " + strings.Join(notes, " ") + "\n")
		}
	}
	b.WriteString("Your task for this reply: " + move.Instruction + "\n")
	return b.String()
}

// d5InterviewerMessages returns the system prompt plus the current mode's last six
// transcript messages. Openings get a short user turn so the chat starts with the user.
func d5InterviewerMessages(system string, sess *d5Session, mode string, opening bool) []chatMessage {
	msgs := []chatMessage{{Role: "system", Content: system}}
	if opening {
		return append(msgs, chatMessage{Role: "user", Content: "I'm ready to begin."})
	}
	history := sess.modeMessages(mode)
	if len(history) > 6 {
		history = history[len(history)-6:]
	}
	for _, m := range history {
		msgs = append(msgs, chatMessage{Role: m.Role, Content: m.Content})
	}
	return msgs
}

func transcriptText(sess *d5Session, mode string) string {
	var b strings.Builder
	name := sess.Personas[mode].Name
	for _, m := range sess.modeMessages(mode) {
		if m.Role == "assistant" {
			fmt.Fprintf(&b, "Interviewer (%s): %s\n\n", name, m.Content)
		} else {
			fmt.Fprintf(&b, "Candidate: %s\n\n", m.Content)
		}
	}
	return strings.TrimSpace(b.String())
}

func targetDescription(dim string) string {
	switch dim {
	case dimConceptual:
		return "conceptual understanding"
	case dimDecomposition:
		return "decomposition (breaking the problem down)"
	case dimCorrectness:
		return "correctness of their code"
	case dimUnderstanding:
		return "understanding of their own code"
	case dimAIUse:
		return "AI use (how they used and checked AI tools)"
	case dimStrategy:
		return "debugging strategy"
	default:
		return dim
	}
}

func progressSentences(state *AgentSessionState, sess *d5Session, mode string) string {
	asked := len(state.ModeQuestionsAsked)
	switch mode {
	case modeConceptual:
		return fmt.Sprintf("Questions asked in this part: %d (3–5 expected; the part closes automatically at %d). Vague answers so far: %d.", asked, maxConceptualQuestions, state.ModeVagueAnswers)
	case modeCode:
		pasted := "no"
		if sess.CodePasted {
			pasted = "yes"
		}
		return fmt.Sprintf("Code pasted: %s. Questions since the paste: %d of up to %d (one should be about AI use). Vague answers so far: %d.", pasted, len(postCodeQuestions(state)), maxPostCodeQuestions, state.ModeVagueAnswers)
	case modeBug:
		return fmt.Sprintf("Questions asked in this part: %d of %d. Vague answers so far: %d.", asked, maxBugQuestions, state.ModeVagueAnswers)
	}
	return ""
}

func briefLines(b *d5Brief) string {
	if b == nil {
		return ""
	}
	var lines []string
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("COVERED", strings.Join(b.Covered, "; "))
	add("GAPS", strings.Join(b.Gaps, "; "))
	add("NEXT", b.Next)
	return strings.Join(lines, "\n")
}

// d5EvaluatorMessages builds the Evaluator call (design §5): rubric and guide first, the
// transcript last. issues are post-check hits on the interviewer's latest reply.
func d5EvaluatorMessages(state *AgentSessionState, sess *d5Session, mode string, answerIndex int, target string, prev *d5Brief, issues []string) []chatMessage {
	var b strings.Builder
	fmt.Fprintf(&b, `You are a senior interviewer reviewing a mock job interview for an entry-level Python role. You never talk to the candidate. You write private notes for the interviewer, in exactly the line format at the end.

Part of the interview: %s. Topic: %s. Candidate's background: %s.
`, modeLabel(mode), state.SelectedKeyConcept, state.StudentMajor)
	if !state.isProblemDecompositionWeek() {
		fmt.Fprintf(&b, "Python ideas the interviewer may use: %s. Never steer toward anything else; dictionaries are never allowed.", allowedPythonIdeas(state.CurrentWeekNumber))
		if loopBanApplies(state.CurrentWeekNumber) {
			b.WriteString(" Never steer toward `while True`, `break` or `continue`.")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nRubric level descriptions for this part (use them to judge; never quote them):\n%s\n\nGuide for this topic:\n%s\n\n", d5RubricForMode(state.CurrentWeekNumber, mode), weekGuide(state.CurrentWeekNumber))
	fmt.Fprintf(&b, "Progress: %s\n", progressSentences(state, sess, mode))
	fmt.Fprintf(&b, "Assess the candidate's latest answer (answer #%d). The question it answered targeted: %s.\n", answerIndex, targetDescription(target))
	if prev != nil && prev.Mode == mode {
		fmt.Fprintf(&b, "Your previous notes:\n%s\n", briefLines(prev))
	}
	if len(issues) > 0 {
		fmt.Fprintf(&b, "Problems detected in the interviewer's latest reply: %s\n", strings.Join(issues, "; "))
	}
	dims := strings.Join(modeDimensions[mode], ", ")
	fmt.Fprintf(&b, `
Write exactly these lines, in this order, and nothing else:
QUALITY: vague | partial | solid | strong
CLARIFICATION: yes | no (did the latest message ask for clarification instead of answering?)
EVIDENCE: what the latest answer showed, as short items separated by ;
COVERED: points the candidate has shown so far in this part, separated by ;
GAPS: points not yet shown, separated by ;
NEXT: one sentence on what the next question should probe, on new ground, without giving answers away
FALLBACK: one complete interview question the interviewer could ask next
AVOID: questions already asked that must not be repeated, separated by ;
RECOMMEND: continue | close (close only when there is enough evidence to judge this part)
LEVELS: dimension=level for the latest answer only. Dimensions: %s. Levels: not_ready, competent, exceptional. Always label the targeted dimension; add others only if the answer clearly shows them.
ISSUES: rule problems in the interviewer's latest reply (more than one question, giving away answers, out-of-scope ideas, teaching), or none
`, dims)
	return []chatMessage{
		{Role: "system", Content: b.String()},
		{Role: "user", Content: "Interview transcript for this part:\n\n" + transcriptText(sess, mode)},
	}
}

// d5LevelsMessages builds the small levels-only call made at mode close (design §8).
func d5LevelsMessages(state *AgentSessionState, mode, target, question, answer string) []chatMessage {
	dims := strings.Join(modeDimensions[mode], ", ")
	system := fmt.Sprintf(`Label one interview answer against the rubric below. Output exactly one line and nothing else:
LEVELS: dimension=level; dimension=level
Dimensions for this part: %s. Levels: not_ready, competent, exceptional.
The question targeted %s: always label that dimension. Add other dimensions only if the answer clearly shows them.

Rubric:
%s`, dims, target, d5RubricForMode(state.CurrentWeekNumber, mode))
	return []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: "Question: " + question + "\nAnswer: " + answer},
	}
}

func modeTitle(mode string) string {
	switch mode {
	case modeConceptual:
		return "Conceptual"
	case modeCode:
		return "Code"
	case modeBug:
		return "Bug hunting"
	}
	return mode
}

// d5CoachingSystemPrompt builds the coaching prompt (design §8): the coach is a mentor at
// the same company and explains the evidence behind each bucket without changing it.
func d5CoachingSystemPrompt(state *AgentSessionState, sess *d5Session) string {
	p := sess.Personas[modeCoaching]
	var b strings.Builder
	company := sess.CompanyName
	if company == "" {
		company = "the company"
	}
	fmt.Fprintf(&b, "You are %s, a mentor at %s. The candidate (%s background) just finished a practice interview for an entry-level role and asked for feedback. You are warm, encouraging and specific, and you frame growth around the job, not school. Never mention weeks, courses or homework.\n\n", p.Name, company, state.StudentMajor)
	fmt.Fprintf(&b, "Their results are final; never change or second-guess them:\n- Conceptual: %s\n- Code: %s\n- Bug hunting: %s\n- Overall: %s\n\n", state.ConceptualAssessmentBucket, state.CodeAssessmentBucket, state.BugAssessmentBucket, state.FinalRating)
	b.WriteString("What the interviewers observed:\n")
	sess.mu.Lock()
	for _, mode := range []string{modeConceptual, modeCode, modeBug} {
		evidence := sess.Evidence[mode]
		var gaps []string
		if brief := sess.ModeBriefs[mode]; brief != nil {
			gaps = brief.Gaps
		}
		if len(evidence) == 0 && len(gaps) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s. Shown: %s. Not yet shown: %s.\n", modeTitle(mode), strings.Join(evidence, "; "), strings.Join(gaps, "; "))
	}
	sess.mu.Unlock()
	fmt.Fprintf(&b, "\nGuide for this topic:\n%s\n\n", weekGuide(state.CurrentWeekNumber))
	if state.isProblemDecompositionWeek() {
		b.WriteString("Coach on breaking problems into inputs, steps and outputs only.\n")
	} else {
		fmt.Fprintf(&b, "Practice suggestions may use only these Python ideas: %s. Never mention dictionaries.", allowedPythonIdeas(state.CurrentWeekNumber))
		if loopBanApplies(state.CurrentWeekNumber) {
			b.WriteString(" Suggest condition-driven `while` loops, never `while True`, `break` or `continue` drills.")
		}
		b.WriteString("\n")
	}
	b.WriteString(`
In your first reply, for each part that was rated: explain briefly what in their answers led to that rating, name 1–3 strengths and 1–3 things to work on, and suggest 1–2 concrete practice actions. End with one check-in question. In later replies, answer their questions the same way. Never give full solutions to interview tasks. Plain text with short paragraphs or bullets, under 250 words.`)
	return b.String()
}
