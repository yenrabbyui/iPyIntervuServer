package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const (
	interviewStepOpening                 = "opening"
	interviewStepAwaitingAnswer          = "awaiting_answer"
	interviewStepInterviewing            = "interviewing"
	interviewStepDecompositionAnswered   = "decomposition_answered"
	interviewStepAwaitingCode            = "awaiting_code"
	interviewStepCodeSubmitted           = "code_submitted"
	interviewStepFollowUp                = "follow_up"
)

const week1ConceptualForwardPolicy = "Week 1 uses ONE scenario for the whole interview. Ask follow-ups about that same scenario only — never present a new scenario. Once the student's answers show their input, process, and output decomposition, finish with conceptualAssessmentPhase complete plus conceptualAssessmentBucket."

func resetModeInterviewProgress(state *AgentSessionState) {
	if state == nil {
		return
	}
	state.ModeInterviewStep = interviewStepOpening
	state.ModeOpeningServed = false
	state.ModeUserAnsweredSinceOpening = false
	state.ModeQuestionsAsked = nil
	state.ModeQuestionsBeforeCode = 0
}

func isFollowUpAssessmentTurn(state *AgentSessionState) bool {
	if state == nil || state.ConversationPhase != phaseAssessmentInProgress {
		return false
	}
	switch state.ActiveMode {
	case modeConceptual, modeCode, modeBug:
		return state.ModeOpeningServed && state.ModeUserAnsweredSinceOpening
	default:
		return false
	}
}

func advanceInterviewProgressOnUserAnswer(state *AgentSessionState) {
	if state == nil || !state.ModeOpeningServed {
		return
	}
	state.ModeUserAnsweredSinceOpening = true

	switch state.ActiveMode {
	case modeConceptual:
		if state.ModeInterviewStep == interviewStepOpening || state.ModeInterviewStep == interviewStepAwaitingAnswer {
			state.ModeInterviewStep = interviewStepInterviewing
		}
	case modeCode:
		switch state.ModeInterviewStep {
		case interviewStepOpening, interviewStepAwaitingAnswer:
			state.ModeInterviewStep = interviewStepDecompositionAnswered
		case interviewStepAwaitingCode:
			if looksLikeCodeSubmission(state.LastUserMessageRaw) {
				state.ModeInterviewStep = interviewStepCodeSubmitted
				state.ModeQuestionsBeforeCode = len(state.ModeQuestionsAsked)
			}
		}
	case modeBug:
		if state.ModeInterviewStep == interviewStepOpening || state.ModeInterviewStep == interviewStepAwaitingAnswer {
			state.ModeInterviewStep = interviewStepFollowUp
		}
	}
}

func updateInterviewProgressAfterAssistant(state *AgentSessionState, assistant string) {
	// Do not require a sync block: a delivered opening without one must still count as
	// served, or every later turn is treated as the opening and rewound to the scenario.
	if state == nil || state.ConversationPhase != phaseAssessmentInProgress || clientVisibleAssistantContent(assistant) == "" {
		return
	}
	switch state.ActiveMode {
	case modeConceptual, modeCode, modeBug:
		if !state.ModeOpeningServed {
			state.ModeOpeningServed = true
			state.ModeInterviewStep = interviewStepAwaitingAnswer
		}
		if q := lastQuestionSentence(clientVisibleAssistantContentGuarded(assistant, state)); q != "" {
			state.ModeQuestionsAsked = append(state.ModeQuestionsAsked, q)
		}
		if state.ActiveMode == modeCode {
			advanceCodeInterviewStepFromAssistant(state, assistant)
		}
	}
}

func advanceCodeInterviewStepFromAssistant(state *AgentSessionState, assistant string) {
	visible := strings.ToLower(strings.TrimSpace(stripIPyIntervuTail(assistant)))
	switch state.ModeInterviewStep {
	case interviewStepDecompositionAnswered, interviewStepInterviewing:
		if looksLikeCodeRequest(visible) {
			state.ModeInterviewStep = interviewStepAwaitingCode
		}
	}
}

func looksLikeCodeRequest(text string) bool {
	for _, phrase := range []string{
		"paste your code",
		"paste the code",
		"share your code",
		"send your code",
		"submit your code",
		"provide your code",
		"paste your solution",
		"paste your python",
		"share your implementation",
		"paste it here",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return codeRequestPattern.MatchString(text)
}

// codeRequestPattern catches wordings the phrase list misses, such as "please paste the
// Python code you wrote". "pasted"/"shared" do not match, so "the code you pasted" is not a request.
var codeRequestPattern = regexp.MustCompile(`(?i)\b(paste|share|send|submit|provide)\s+(me\s+)?(your|the|that|it)?\s*(python\s+)?(code|solution|script)\b`)

func looksLikeCodeSubmission(userMessage string) bool {
	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "```") {
		return true
	}
	lower := strings.ToLower(msg)
	for _, token := range []string{"def ", "import ", "print(", "for ", "while ", "if ", "input("} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return len(msg) > 120 && strings.Count(msg, "\n") >= 2
}

func forwardInterviewMoveGuidance(state *AgentSessionState) string {
	if state == nil {
		return "Continue with one appropriate interview question for the active mode."
	}
	switch state.ActiveMode {
	case modeCode:
		switch state.ModeInterviewStep {
		case interviewStepDecompositionAnswered, interviewStepInterviewing, interviewStepFollowUp:
			return "The student already answered decomposition. Your next move MUST ask them to paste their Python code for the task (e.g. 'Please paste your Python code.'). Do not repeat the opening decomposition question and do not send complete until pasted code is received and assessed."
		case interviewStepAwaitingCode:
			return "You asked for code. Wait for pasted Python code if not yet received — do not repeat decomposition. After paste, evaluate code and ask explain-code or AI-use questions before complete."
		case interviewStepCodeSubmitted:
			return "The student already pasted their code — never ask for it again. Ask one explain-code, line-level, or AI-use reflection question — do not repeat decomposition. Once the student has explained the code and answered an AI-use question, finish with codeAssessmentPhase complete plus codeAssessmentBucket."
		default:
			return "Continue the code interview with the next step (code request or explain-code). Do not repeat the opening decomposition question."
		}
	case modeBug:
		if state.ModeInterviewStep == interviewStepFollowUp || state.ModeInterviewStep == interviewStepInterviewing {
			return "Continue bug hunting with one new debugging-process follow-up question. Do not repeat the opening scenario question verbatim."
		}
		return "Continue bug hunting with one debugging-process question. Do not repeat the opening scenario question verbatim."
	case modeConceptual:
		if state.isProblemDecompositionWeek() {
			if remaining := decompositionPartsRemaining(state); isFollowUpAssessmentTurn(state) && len(remaining) > 0 {
				return week1ConceptualForwardPolicy + " Parts the student has not been asked about yet: " + strings.Join(remaining, ", ") + "."
			}
			return week1ConceptualForwardPolicy
		}
		return "Continue with one new conceptual interview question. Do not repeat the previous question verbatim."
	default:
		return "Continue with one interview question appropriate to the active mode."
	}
}

func interviewProgressSnapshot(state *AgentSessionState) map[string]any {
	if state == nil || state.ConversationPhase != phaseAssessmentInProgress {
		return nil
	}
	switch state.ActiveMode {
	case modeConceptual, modeCode, modeBug:
	default:
		return nil
	}
	snap := map[string]any{
		"step":                      state.ModeInterviewStep,
		"openingServed":             state.ModeOpeningServed,
		"userAnsweredSinceOpening": state.ModeUserAnsweredSinceOpening,
	}
	if len(state.ModeQuestionsAsked) > 0 {
		snap["questionsAsked"] = state.ModeQuestionsAsked
	}
	if state.ActiveMode == modeConceptual && state.isProblemDecompositionWeek() {
		snap["decompositionPartsAnswered"] = decompositionPartsAnswered(state)
	}
	switch state.ActiveMode {
	case modeCode:
		if state.ModeInterviewStep == interviewStepCodeSubmitted {
			snap["forwardPolicy"] = "The student has pasted their code — never ask for it again. Ask explain-code and AI-use questions, then finish with codeAssessmentPhase complete plus codeAssessmentBucket."
		} else {
			snap["forwardPolicy"] = "After decomposition is answered, explicitly ask the student to paste their Python code — then evaluate pasted code before complete. Never repeat the opening decomposition ask or complete after decomposition alone."
		}
	case modeBug:
		snap["forwardPolicy"] = "After the opening debug question is answered, ask follow-up debugging-process questions — never repeat the opening question verbatim."
	case modeConceptual:
		if state.isProblemDecompositionWeek() {
			snap["forwardPolicy"] = week1ConceptualForwardPolicy
		} else {
			snap["forwardPolicy"] = "Ask new conceptual follow-ups; do not repeat the same question verbatim."
		}
	}
	return snap
}

// followUpTurnDirective states the interview position plainly for this turn. The
// interviewProgress flags alone did not stop the model from re-introducing a persona,
// re-asking answered questions, or interviewing past the point it should close.
func followUpTurnDirective(state *AgentSessionState) string {
	if !isFollowUpAssessmentTurn(state) {
		return ""
	}
	if modeClosingDue(state) {
		phaseField, bucketField, _ := currentModeSyncFields(state)
		return "THIS TURN MUST CLOSE THE INTERVIEW: the student has answered enough questions (" + closingDueReason(state) +
			"). Ask no question and do not ask for code. Send one brief neutral closing sentence and end with ```_ipyintervu``` containing \"" + phaseField + "\": \"complete\" and \"" +
			bucketField + "\" (Not Ready Yet, Competent, or Exceptional) based on all of the student's answers.\n"
	}
	return "THIS TURN: the scenario is already on screen and the student's latest message answers your previous question. The student has already answered: " +
		quotedQuestions(state.ModeQuestionsAsked) +
		". Do not welcome the student again, restate the scenario, or ask any of those questions again in any wording (unless the student asked you to clarify one). A persona speaking for the first time gives at most one short clause of introduction. " +
		forwardInterviewMoveGuidance(state) + "\n"
}

func quotedQuestions(questions []string) string {
	if len(questions) == 0 {
		return "your previous question"
	}
	quoted := make([]string, len(questions))
	for i, q := range questions {
		quoted[i] = fmt.Sprintf("%q", q)
	}
	return strings.Join(quoted, "; ")
}

// maxConceptualQuestions is the top of the conceptual mode's 3–5 question guideline.
const maxConceptualQuestions = 5

var decompositionParts = []string{"input", "process", "output"}

// decompositionPart classifies a Week 1 question by the first part it names, so
// "what is the process — the steps from those inputs to a result?" counts as process.
func decompositionPart(question string) string {
	lower := strings.ToLower(question)
	best, bestIdx := "", -1
	for part, keywords := range map[string][]string{
		"input":   {"input"},
		"process": {"process", "steps"},
		"output":  {"output"},
	} {
		for _, kw := range keywords {
			if idx := strings.Index(lower, kw); idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				best, bestIdx = part, idx
			}
		}
	}
	return best
}

// decompositionPartsAnswered lists the Week 1 parts the student has been asked about. Every
// recorded question has been answered by the time a follow-up turn is built.
func decompositionPartsAnswered(state *AgentSessionState) []string {
	asked := map[string]bool{}
	for _, q := range state.ModeQuestionsAsked {
		asked[decompositionPart(q)] = true
	}
	var answered []string
	for _, part := range decompositionParts {
		if asked[part] {
			answered = append(answered, part)
		}
	}
	return answered
}

func decompositionPartsRemaining(state *AgentSessionState) []string {
	answered := strings.Join(decompositionPartsAnswered(state), " ")
	var remaining []string
	for _, part := range decompositionParts {
		if !strings.Contains(answered, part) {
			remaining = append(remaining, part)
		}
	}
	return remaining
}

// conceptualClosingDue reports that this conceptual turn must be the closing reply: Week 1
// once input, process, and output are all answered, any week after the question cap.
func conceptualClosingDue(state *AgentSessionState) bool {
	if state.ActiveMode != modeConceptual || !isFollowUpAssessmentTurn(state) || studentAskedForClarification(state.LastUserMessageRaw) {
		return false
	}
	if state.isProblemDecompositionWeek() && len(decompositionPartsRemaining(state)) == 0 {
		return true
	}
	return len(state.ModeQuestionsAsked) >= maxConceptualQuestions
}

// maxPostCodeQuestions caps the explain-code and AI-use questions asked after the paste.
const maxPostCodeQuestions = 3

// postCodeQuestions lists the questions asked since the student pasted their code.
func postCodeQuestions(state *AgentSessionState) []string {
	if state.ModeQuestionsBeforeCode > len(state.ModeQuestionsAsked) {
		return nil
	}
	return state.ModeQuestionsAsked[state.ModeQuestionsBeforeCode:]
}

var aiUsePattern = regexp.MustCompile(`(?i)\b(ai|a\.i\.|chatgpt|copilot|gemini|claude|llm)\b|artificial intelligence`)

func askedAboutAIUse(questions []string) bool {
	for _, q := range questions {
		if aiUsePattern.MatchString(q) {
			return true
		}
	}
	return false
}

// codeClosingDue reports that this code turn must be the closing reply: the student has
// pasted code and answered an explain-code question plus an AI-use question, or the cap.
// Without it the model kept interviewing and eventually asked for the code again.
func codeClosingDue(state *AgentSessionState) bool {
	if state.ActiveMode != modeCode || state.ModeInterviewStep != interviewStepCodeSubmitted ||
		!isFollowUpAssessmentTurn(state) || studentAskedForClarification(state.LastUserMessageRaw) {
		return false
	}
	asked := postCodeQuestions(state)
	return len(asked) >= maxPostCodeQuestions || (len(asked) >= 2 && askedAboutAIUse(asked))
}

// modeClosingDue reports that the active mode's interview is finished and this turn must close it.
func modeClosingDue(state *AgentSessionState) bool {
	return conceptualClosingDue(state) || codeClosingDue(state)
}

func closingDueReason(state *AgentSessionState) string {
	if state.ActiveMode == modeCode {
		return fmt.Sprintf("code pasted and %d questions about it answered", len(postCodeQuestions(state)))
	}
	if state.isProblemDecompositionWeek() && len(decompositionPartsRemaining(state)) == 0 {
		return "input, process, and output have all been answered"
	}
	return fmt.Sprintf("%d questions asked", len(state.ModeQuestionsAsked))
}

// lastQuestionSentence returns the final question sentence of visible assistant text.
func lastQuestionSentence(visible string) string {
	visible = strings.TrimSpace(stripCodeFences(visible))
	end := strings.LastIndex(visible, "?")
	if end < 0 {
		return ""
	}
	start := strings.LastIndexAny(visible[:end], ".!?\n")
	return strings.TrimSpace(visible[start+1 : end+1])
}

var questionStopWords = map[string]bool{
	"what": true, "would": true, "you": true, "your": true, "identify": true, "the": true, "this": true,
	"that": true, "these": true, "those": true, "for": true, "and": true, "any": true, "are": true,
	"can": true, "could": true, "how": true, "which": true, "when": true, "where": true, "why": true,
	"with": true, "from": true, "into": true, "about": true, "does": true, "did": true, "should": true,
	"might": true, "will": true, "there": true, "here": true, "have": true, "has": true, "our": true,
	"we": true, "consider": true, "think": true, "say": true, "describe": true, "explain": true,
	"begin": true, "begins": true, "start": true, "starts": true, "task": true, "then": true,
}

func questionContentWords(question string) map[string]bool {
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) < 3 || questionStopWords[w] {
			continue
		}
		for _, suffix := range []string{"ing", "es", "ed", "s"} {
			if len(w) > len(suffix)+3 && strings.HasSuffix(w, suffix) {
				w = strings.TrimSuffix(w, suffix)
				break
			}
		}
		words[w] = true
	}
	return words
}

// isRepeatedQuestion reports whether next asks essentially the same thing as prev: at least
// two shared content words covering most of the shorter question. "What is the input?" and
// "What is the output?" share none, so asking for the next decomposition part is not a repeat.
func isRepeatedQuestion(prev, next string) bool {
	a, b := questionContentWords(prev), questionContentWords(next)
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	shared := 0
	for w := range a {
		if b[w] {
			shared++
		}
	}
	smaller := len(a)
	if len(b) < smaller {
		smaller = len(b)
	}
	return shared >= 2 && float64(shared)/float64(smaller) >= 0.6
}

func studentAskedForClarification(userMessage string) bool {
	lower := strings.ToLower(userMessage)
	if strings.Contains(lower, "?") {
		return true
	}
	for _, phrase := range []string{"don't understand", "dont understand", "not sure what you mean", "confused", "clarify", "rephrase"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// repeatsAnsweredQuestion reports a follow-up reply that re-asks any question the student
// already answered in this mode (e.g. asking for the input again after the output).
func repeatsAnsweredQuestion(state *AgentSessionState, assistant string) bool {
	if state.ActiveMode == modeCode && state.ModeInterviewStep == interviewStepCodeSubmitted && isFollowUpAssessmentTurn(state) &&
		looksLikeCodeRequest(strings.ToLower(clientVisibleAssistantContent(assistant))) {
		return true
	}
	// Code mode legitimately re-asks for code the student has not pasted yet.
	if (state.ActiveMode == modeCode && state.ModeInterviewStep != interviewStepCodeSubmitted) || !isFollowUpAssessmentTurn(state) || len(state.ModeQuestionsAsked) == 0 || studentAskedForClarification(state.LastUserMessageRaw) {
		return false
	}
	candidates := studentDirectedInterviewQuestions(assistant)
	if next := lastQuestionSentence(clientVisibleAssistantContent(assistant)); next != "" {
		candidates = append(candidates, next)
	}
	for _, prev := range state.ModeQuestionsAsked {
		for _, q := range candidates {
			if isRepeatedQuestion(prev, q) {
				return true
			}
		}
	}
	return false
}
