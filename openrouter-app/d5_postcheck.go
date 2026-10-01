package main

import (
	"regexp"
	"strings"
)

var d5ClarificationPhrases = append([]string{
	"what do you mean", "can you explain the question", "could you explain the question",
	"repeat the question", "say that again", "what are you asking", "which part",
}, clarificationPhrases...)

// answerVoicePattern marks a message that commits to an answer ("I'd add a print"), even
// when phrased as a question.
var answerVoicePattern = regexp.MustCompile(`(?i)\bi(?:'d| would| will|'ll| think| guess| believe| might)\b`)

// d5AskedForClarification is the tightened clarification check (design §6): a
// clarification phrase in a short message, or a short message (≤ 20 words) that is mainly
// a question back to the interviewer. Any "?" no longer counts, so "Would I check the
// input first? I'd add a print…" stays an answer.
func d5AskedForClarification(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" || d5LooksLikeCodeSubmission(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	words := len(strings.Fields(lower))
	if words <= 25 && containsAny(lower, d5ClarificationPhrases) {
		return true
	}
	return words <= 20 && strings.HasSuffix(lower, "?") && !answerVoicePattern.MatchString(lower)
}

// pythonStatementLine matches a line that reads as a Python statement rather than prose.
var pythonStatementLine = regexp.MustCompile(`^\s*(?:def \w+\(|import \w|from \w+ import|(?:for|while|if|elif) .+:\s*$|else:\s*$|return\b|print\(|[A-Za-z_]\w*(?:\[[^\]]*\])?\s*[-+*/]?=[^=])`)

// d5LooksLikeCodeSubmission reports pasted code. It is stricter than looksLikeCodeSubmission,
// which treats any "if " or "for " as code, so a prose decomposition ("apply the discount
// if it is over 50") would count as a paste and skip the request for code.
func d5LooksLikeCodeSubmission(msg string) bool {
	if strings.Contains(msg, "```") {
		return true
	}
	lines := 0
	for _, line := range strings.Split(msg, "\n") {
		if pythonStatementLine.MatchString(line) {
			lines++
		}
	}
	if lines >= 2 {
		return true
	}
	return lines == 1 && len(strings.Split(strings.TrimSpace(msg), "\n")) == 1 && strings.ContainsAny(msg, "()")
}

// d5ScopeDetectors are scopeDetectors with two false positives fixed (design §11):
// prose "user input" is fine in Week 1 decomposition (handled in d5OutOfScopeConcepts),
// and prose "file(s)" alone is ordinary business wording, not week 9 file I/O.
var d5ScopeDetectors = func() []scopeDetector {
	out := make([]scopeDetector, len(scopeDetectors))
	copy(out, scopeDetectors)
	for i := range out {
		if out[i].week == 9 {
			out[i].prose = scopePatterns(
				`\b(?:read|reads|reading|write|writes|writing|open|opens|opening|save|saves|saving|load|loads|loading)\b[^.?!\n]{0,30}\b(?:text |data |csv )?files?\b`,
				`\bfile (?:i/o|io|handling)\b`, `\bopen\s*\(`, `\bcsv\b`, `\.(?:read|readline|readlines|write)\s*\(`, `\.txt\b`,
			)
		}
	}
	return out
}()

var week3ProseInputPattern = regexp.MustCompile(`(?i)\buser (?:will |would |should |can )?(?:enters?|types?|inputs?)\b|\buser input\b|\b(?:prompts?|asks?) the user\b`)

// d5OutOfScopeConcepts is outOfScopeConcepts with the D5 detector fixes.
func d5OutOfScopeConcepts(currentWeek int, visible string) []string {
	if currentWeek < 1 {
		return nil
	}
	code := replyCodeSegments(visible)
	var found []string
	for _, d := range d5ScopeDetectors {
		if d.week <= currentWeek {
			continue
		}
		prose := visible
		if currentWeek == 1 && d.week == 3 {
			// Week 1 scenarios talk about what a user enters; that is decomposition, not input().
			prose = week3ProseInputPattern.ReplaceAllString(prose, " ")
		}
		if matchesAny(d.prose, prose) || (code != "" && matchesAny(d.code, code)) {
			if d.week == neverInScopeWeek {
				found = append(found, d.name+" (never in scope)")
			} else {
				found = append(found, d.name+" (later week)")
			}
		}
	}
	return found
}

var (
	loopBanCodePattern  = regexp.MustCompile(`(?i)\bwhile\s+true\b|\bbreak\b|\bcontinue\b`)
	loopBanProsePattern = regexp.MustCompile(`(?i)\bwhile true\b|\bbreak statements?\b|\bcontinue statements?\b`)
)

// promptsBannedLoopControl reports a reply that brings up `while True`, `break` or
// `continue` in weeks 7–9 when the candidate has not used them in this mode.
func promptsBannedLoopControl(week int, reply string, candidateText string) bool {
	if !loopBanApplies(week) {
		return false
	}
	mentioned := loopBanCodePattern.MatchString(replyCodeSegments(reply)) || loopBanProsePattern.MatchString(reply)
	if !mentioned {
		return false
	}
	return !loopBanCodePattern.MatchString(candidateText)
}

// correctnessVerdictPattern is evaluativePraisePattern narrowed to verdicts on whether an
// answer was right; warm phrasing ("Thanks, that's helpful") is allowed (design §9).
var correctnessVerdictPattern = regexp.MustCompile(`(?i)\b(?:that's|that is) (?:exactly |absolutely |totally )?(?:right|correct)\b|\bexactly right\b|\byou(?:'re| are) (?:absolutely |exactly |totally )?(?:right|correct)\b|\bspot on\b|\bnailed it\b|(?:^|\n)\s*(?:exactly|correct)\b`)

var (
	d5PersonaGreetingPattern = regexp.MustCompile(`(?i)^\s*(?:hey|hi|hello|thanks|thank you)[,\s]+(?:` + d5PersonaNamePattern + `)\s*[,.!:—–-]`)
	d5PersonaAddressPattern  = regexp.MustCompile(`(?i)^\s*(?:so|and|now|ok(?:ay)?)?[,\s]*(?:` + d5PersonaNamePattern + `)\s*[,:—–-]|[,—–-]\s*(?:` + d5PersonaNamePattern + `)\s*[?.!]*\s*$`)
)

// d5AddressesPersona reports a sentence that greets or questions another interviewer,
// using the pooled persona names. The current speaker introducing themselves is fine.
func d5AddressesPersona(reply, speaker string) bool {
	for _, sentence := range replySentencePattern.FindAllString(stripCodeFences(reply), -1) {
		if speaker != "" && strings.Contains(strings.ToLower(sentence), "i'm "+strings.ToLower(speaker)) {
			continue
		}
		if d5PersonaGreetingPattern.MatchString(sentence) || (strings.Contains(sentence, "?") && d5PersonaAddressPattern.MatchString(sentence)) {
			return true
		}
	}
	return false
}

// d5SpeaksAsInstructor is speaksAsInstructor without the education-major exception.
func d5SpeaksAsInstructor(reply string) bool {
	prose := stripCodeFences(reply)
	return classroomFramingPattern.MatchString(prose) || instructorRolePattern.MatchString(prose)
}

// d5PostCheckResult is the outcome of the after-the-fact checks on one Interviewer reply.
type d5PostCheckResult struct {
	Reply  string
	Issues []string
}

// d5PostCheck runs the §9 checks on a finished Interviewer reply. It never triggers a
// retry: self-answers are truncated, a reply with no question gets the fallback question
// appended, and everything else is reported for the next Evaluator run.
func d5PostCheck(state *AgentSessionState, sess *d5Session, mode string, move d5Move, reply, fallbackQuestion string) d5PostCheckResult {
	res := d5PostCheckResult{Reply: strings.TrimSpace(reply)}
	add := func(issue string) { res.Issues = append(res.Issues, issue) }

	if !move.isOpening() && looksLikeSelfAnsweredQuestion(res.Reply) {
		res.Reply = strings.TrimSpace(guardQuestionOnlyResponse(res.Reply))
		add("answered its own question or wrote the candidate's reply")
	}
	// A request for code ("please write … and paste it here") asks something without a "?".
	if !asksStudentSomething(res.Reply) && !looksLikeCodeRequest(strings.ToLower(res.Reply)) && fallbackQuestion != "" {
		res.Reply = strings.TrimSpace(res.Reply + "\n\n" + fallbackQuestion)
		add("asked the candidate nothing")
	}
	if countInterviewQuestions(res.Reply) > 1 {
		add("asked more than one question")
	}
	if correctnessVerdictPattern.MatchString(stripCodeFences(res.Reply)) {
		add("told the candidate whether they were right")
	}
	if leaksReasoning(res.Reply) {
		add("used internal wording (the student, rubric, portion)")
	}
	if d5AddressesPersona(res.Reply, sess.Personas[mode].Name) {
		add("addressed another interviewer instead of the candidate")
	}
	if !move.isOpening() && move.Kind != moveClarify && move.Kind != moveRequestCode {
		if q := lastQuestionSentence(res.Reply); q != "" && repeatsRecordedQuestion(state, q) {
			add("re-asked a question already answered")
		}
	}
	lower := strings.ToLower(res.Reply)
	if (mode == modeCode && sess.CodePasted && looksLikeCodeRequest(lower)) || (mode == modeBug && looksLikeCodeRequest(lower)) {
		add("asked for code again or for fixed code")
	}
	if found := d5OutOfScopeConcepts(state.CurrentWeekNumber, res.Reply); len(found) > 0 {
		add("mentioned out-of-scope ideas: " + strings.Join(found, ", "))
	}
	var candidate strings.Builder
	for _, m := range sess.modeMessages(mode) {
		if m.Role == "user" {
			candidate.WriteString(m.Content + "\n")
		}
	}
	if promptsBannedLoopControl(state.CurrentWeekNumber, res.Reply, candidate.String()) {
		add("brought up while True, break or continue")
	}
	if hasUnfilledPlaceholder(res.Reply) {
		add("left a template placeholder")
	}
	if d5SpeaksAsInstructor(res.Reply) {
		add("spoke as an instructor or mentioned the course")
	}
	return res
}
