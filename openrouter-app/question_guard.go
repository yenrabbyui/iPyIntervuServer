package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	simulatedStudentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(student|you|candidate|applicant|user)\s*:\s*`),
		regexp.MustCompile(`(?i)\b(your answer(?: would be)?|the answer is|correct answer|expected answer|sample answer)\s*:`),
	}
	selfAnswerPhrasePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(this means|that means|it works by|works by|you would|you could|you should|you'd|so you can|for example)\b`),
		regexp.MustCompile(`(?i)\b(in python,?|append adds|extend adds|the (?:bug|issue|problem|defect|fix) is)\b`),
		regexp.MustCompile(`(?i)\b(that's because|because it|the reason is|so the (?:list|code|program|output))\b`),
		regexp.MustCompile(`(?i)^(?:well|so|basically|simply put|in short),?\s+`),
		regexp.MustCompile(`(?i)^(?:a|an|the)\s+[a-z][a-z\s]{0,40}\s+(?:is|are|means|refers to|stores|holds|adds|removes|iterates|returns)\b`),
		regexp.MustCompile(`(?i)^[a-z][a-z\s]{0,30}\s+(?:is|are|lets you|allows you|enables you|adds|removes|stores|holds|means|refers to)\b`),
	}
	codeFencePattern = regexp.MustCompile("(?s)```.*?```")
)

func questionGuardEnabledForState(state *AgentSessionState) bool {
	if state == nil {
		return false
	}
	if state.ConversationPhase != phaseAssessmentInProgress {
		return false
	}
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return false
	}
	if isActiveModeComplete(state) {
		return false
	}
	switch state.ActiveMode {
	case modeConceptual, modeCode, modeBug:
		return true
	default:
		return false
	}
}

func findSimulatedStudentIndex(text string) int {
	best := -1
	for _, pattern := range simulatedStudentPatterns {
		if loc := pattern.FindStringIndex(text); loc != nil && (best < 0 || loc[0] < best) {
			best = loc[0]
		}
	}
	return best
}

func stripCodeFences(text string) string {
	return strings.TrimSpace(codeFencePattern.ReplaceAllString(text, " "))
}

func countSentences(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := 0
	start := 0
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			segment := strings.TrimSpace(text[start : i+1])
			if segment != "" {
				count++
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		count++
	}
	return count
}

func isLikelyAnswerContent(after string) bool {
	after = strings.TrimSpace(after)
	if after == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(after), "for example") {
		return true
	}
	if len(after) < 20 {
		return false
	}
	if findSimulatedStudentIndex(after) >= 0 {
		return true
	}
	if strings.Contains(after, "```") {
		return true
	}
	lower := strings.ToLower(after)
	for _, pattern := range selfAnswerPhrasePatterns {
		if pattern.MatchString(after) {
			return true
		}
	}
	if strings.Contains(lower, "the answer") || strings.Contains(lower, "you would ") {
		return true
	}
	if isSimulatedStudentAnswerLine(after) {
		return true
	}
	if countSentences(after) >= 2 && !strings.Contains(after, "?") {
		return true
	}
	runes := []rune(after)
	if len(runes) >= 80 && !strings.HasSuffix(strings.TrimSpace(after), "?") {
		first := runes[0]
		if unicode.IsUpper(first) && countSentences(after) >= 1 {
			for _, pattern := range selfAnswerPhrasePatterns {
				if pattern.MatchString(after) {
					return true
				}
			}
			if strings.Contains(lower, " is ") || strings.Contains(lower, " are ") {
				return true
			}
		}
	}
	return false
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isSimulatedStudentAnswerLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || strings.Contains(line, "?") {
		return false
	}
	if len(line) < 12 {
		return false
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "good point") ||
		strings.HasPrefix(lower, "thanks") ||
		strings.HasPrefix(lower, "got it") ||
		strings.HasPrefix(lower, "understood") ||
		strings.HasPrefix(lower, "okay") ||
		strings.HasPrefix(lower, "ok ") ||
		strings.HasPrefix(lower, "to clarify") {
		return false
	}
	if simulatedStudentAnswerLinePattern.MatchString(line) {
		return true
	}
	if simulatedStudentShortAnswerPattern.MatchString(line) {
		return true
	}
	if first, _ := utf8.DecodeRuneInString(line); first != utf8.RuneError && unicode.IsLower(first) {
		return true
	}
	return false
}

func hasSimulatedStudentLineBetweenQuestions(visible string) bool {
	visible = strings.TrimSpace(stripCodeFences(stripIPyIntervuTail(visible)))
	if visible == "" {
		return false
	}
	parts := strings.Split(visible, "?")
	if len(parts) < 2 {
		return false
	}
	for i := 0; i < len(parts)-1; i++ {
		between := strings.TrimSpace(parts[i+1])
		if between == "" {
			continue
		}
		if isSimulatedStudentAnswerLine(firstNonEmptyLine(between)) {
			return true
		}
	}
	return false
}

func findQuestionAnswerBoundaryInClean(clean string) int {
	searchFrom := 0
	for {
		rel := strings.Index(clean[searchFrom:], "?")
		if rel < 0 {
			return -1
		}
		qIdx := searchFrom + rel
		after := strings.TrimSpace(clean[qIdx+1:])
		if after == "" {
			searchFrom = qIdx + 1
			continue
		}
		nextQRel := strings.Index(after, "?")
		between := after
		if nextQRel >= 0 {
			between = strings.TrimSpace(after[:nextQRel])
		}
		if isLikelyAnswerContent(between) || isSimulatedStudentAnswerLine(firstNonEmptyLine(between)) {
			return qIdx
		}
		if nextQRel < 0 {
			return -1
		}
		searchFrom = qIdx + 1 + nextQRel + 1
	}
}

// looksLikeSelfAnsweredQuestion reports whether visible assistant text asks a question
// and then continues with answer-like content before the student responds.
func looksLikeSelfAnsweredQuestion(visible string) bool {
	visible = strings.TrimSpace(visible)
	if visible == "" {
		return false
	}
	if findSimulatedStudentIndex(visible) >= 0 {
		return true
	}
	if hasSimulatedStudentLineBetweenQuestions(visible) {
		return true
	}
	return findQuestionAnswerBoundaryInClean(stripCodeFences(visible)) >= 0
}

// guardQuestionOnlyResponse truncates visible assistant text so only the question remains.
func guardQuestionOnlyResponse(visible string) string {
	visible = strings.TrimSpace(visible)
	if visible == "" {
		return visible
	}
	if idx := findSimulatedStudentIndex(visible); idx >= 0 {
		return strings.TrimRight(visible[:idx], " \t\n\r")
	}
	clean := stripCodeFences(visible)
	if idx := findQuestionAnswerBoundaryInClean(clean); idx >= 0 {
		return strings.TrimRight(clean[:idx+1], " \t\n\r")
	}
	return visible
}

func guardAssessmentContentResponse(visible string) string {
	visible = strings.TrimSpace(guardQuestionOnlyResponse(visible))
	if countStudentDirectedQuestions(visible) <= 1 {
		return visible
	}
	questions := studentDirectedInterviewQuestions(visible)
	if len(questions) == 0 {
		return visible
	}
	first := questions[0]
	if idx := strings.Index(visible, first); idx >= 0 {
		return strings.TrimRight(visible[:idx+len(first)], " \t\n\r")
	}
	return visible
}

func clientVisibleAssistantContentGuarded(accumulated string, state *AgentSessionState) string {
	visible := clientVisibleAssistantContent(accumulated)
	if !questionGuardEnabledForState(state) {
		return visible
	}
	return guardAssessmentContentResponse(visible)
}

func countStudentDirectedQuestions(visible string) int {
	return len(studentDirectedInterviewQuestions(visible))
}

func countInterviewQuestions(visible string) int {
	return countStudentDirectedQuestions(visible)
}

var studentDirectedInterviewQuestionPattern = regexp.MustCompile(`(?i)\b(?:what would you|how would you|how might you|what might you|what (?:do you|would you|are|is)|how (?:do you|would you|might you|are|is)|which|where would you|when would you|would you|can you|could you|do you|are you|identify|describe|explain|list|name|consider)\b[^.?\n]{0,200}\?`)

func studentDirectedInterviewQuestions(visible string) []string {
	visible = stripCodeFences(stripIPyIntervuTail(visible))
	return studentDirectedInterviewQuestionPattern.FindAllString(visible, -1)
}

var simulatedStudentAnswerLinePattern = regexp.MustCompile(`(?i)^(a|an|the)\s+(table|list|report|chart|summary|output|result|spreadsheet|document|file|map|set)\b`)

var simulatedStudentShortAnswerPattern = regexp.MustCompile(`(?i)^(the )?(type|value|result|answer|output) (would be|is|was)\b`)

var neutralAssessmentLeadInPattern = regexp.MustCompile(`(?i)(?:^|\n)\s*(?:got it\.|thanks\.|understood\.|okay\.)\s`)

// looksLikeCompositeAssessmentReply reports stacked interview content in one reply.
func looksLikeCompositeAssessmentReply(visible string) bool {
	visible = strings.TrimSpace(stripIPyIntervuTail(visible))
	if visible == "" {
		return false
	}
	if hasSimulatedStudentLineBetweenQuestions(visible) {
		return true
	}
	return countStudentDirectedQuestions(visible) > 1
}

// looksLikeSevereCompositeReply reports stacked mini-interviews: simulated student
// answers, multiple questions, or multiple neutral lead-ins in one reply.
func looksLikeSevereCompositeReply(visible string) bool {
	visible = strings.TrimSpace(stripCodeFences(stripIPyIntervuTail(visible)))
	if visible == "" {
		return false
	}
	if hasSimulatedStudentLineBetweenQuestions(visible) {
		return true
	}
	if countStudentDirectedQuestions(visible) > 1 {
		return true
	}
	return countNeutralAssessmentLeadIns(visible) > 1
}

func countNeutralAssessmentLeadIns(text string) int {
	return len(neutralAssessmentLeadInPattern.FindAllStringIndex(text, -1))
}

func isCorrectiveFollowUpKind(kind string) bool {
	return kind == "corrective_retry"
}

func buildDisplayAssistantRaw(handoffParts []string, lastAssistant string) string {
	if len(handoffParts) == 0 {
		return lastAssistant
	}
	if strings.TrimSpace(lastAssistant) == "" {
		return strings.Join(handoffParts, "\n\n")
	}
	return strings.Join(handoffParts, "\n\n") + "\n\n" + lastAssistant
}
