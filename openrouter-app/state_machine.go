package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var (
	bucketPatterns = map[string]*regexp.Regexp{
		"conceptualAssessmentBucket": regexp.MustCompile(`(?i)conceptualAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional)`),
		"codeAssessmentBucket":       regexp.MustCompile(`(?i)codeAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional|N/A)`),
		"bugAssessmentBucket":        regexp.MustCompile(`(?i)bugAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional|N/A)`),
		"businessDomain":             regexp.MustCompile(`(?i)businessDomain\s*[=:]\s*(.+)`),
	}
	jsonBucketPatterns = map[string]*regexp.Regexp{
		"conceptualAssessmentBucket": regexp.MustCompile(`(?i)"conceptualAssessmentBucket"\s*:\s*"(Not Ready Yet|Competent|Exceptional)"`),
		"codeAssessmentBucket":       regexp.MustCompile(`(?i)"codeAssessmentBucket"\s*:\s*"(Not Ready Yet|Competent|Exceptional|N/A)"`),
		"bugAssessmentBucket":        regexp.MustCompile(`(?i)"bugAssessmentBucket"\s*:\s*"(Not Ready Yet|Competent|Exceptional|N/A)"`),
	}
	ipyintervuBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipy(?:intervu)?\\s*\\n(.*?)\\n```")
	partialIPyIntervuOpenPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipy(?:intervu)?\\s*\\n([\\s\\S]+)$")
	proseConceptualBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)conceptual\s+(?:understanding\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+conceptual\s+(?:understanding\s+)?(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great)`),
		regexp.MustCompile(`(?i)conceptual[^.\n]{0,80}\b(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great)\b`),
	}
	proseCodeBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)code\s+(?:problem\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+code\s+(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)`),
		regexp.MustCompile(`(?i)code[^.\n]{0,80}\b(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)\b`),
	}
	proseBugBucketPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)bug\s+(?:hunting\s+)?(?:assessment\s+)?(?:level|rating|bucket)?(?:\s+is|\s*:\s*|\*\*)\s*(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)`),
		regexp.MustCompile(`(?i)(?:your|the)\s+bug\s+(?:assessment\s+)?(?:is\s+)?(?:rated\s+)?(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)`),
		regexp.MustCompile(`(?i)bug[^.\n]{0,80}\b(Not Ready Yet|Not Yet Ready|Competent|Looking Good|Exceptional|Looks Great|N/A)\b`),
	}
)

type ipyintervuTail struct {
	ConceptualAssessmentBucket string `json:"conceptualAssessmentBucket"`
	CodeAssessmentBucket       string `json:"codeAssessmentBucket"`
	BugAssessmentBucket        string `json:"bugAssessmentBucket"`
	BusinessDomain             any    `json:"businessDomain"`
	ActiveMode                 string `json:"activeMode"`
}

func truncateSummary(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func applyPreChatUserUpdate(state *AgentSessionState, userMessage string) {
	state.MessageIndex++
	state.LastUserMessageRaw = userMessage
	state.FirstUserMessageSeen = true

	switch state.ConversationPhase {
	case phaseAwaitingMajor:
		if state.StudentMajor == "" && !isGreetingMessage(userMessage) && strings.TrimSpace(userMessage) != "" {
			state.StudentMajor = strings.TrimSpace(userMessage)
			state.ConversationPhase = phaseAwaitingKeyConcept
			state.PendingQuestion = "weeklyKeyConceptSelection"
			state.WaitingForUserResponse = true
		}
	case phaseAwaitingKeyConcept:
		if sel, ok := matchWeekSelection(userMessage); ok {
			state.SelectedKeyConcept = sel.SelectedKeyConcept
			state.CurrentWeekNumber = sel.CurrentWeekNumber
			state.ConversationPhase = phaseAssessmentInProgress
			state.ActiveMode = modeConceptual
			state.ModesCompleted = nil
			state.PendingQuestion = "conceptualAssessment"
			state.WaitingForUserResponse = true
			state.CoachingRequested = false
			state.CoachingEnteredBeforeResults = false
			markAssessmentStarted(state)
		}
	case phaseAssessmentInProgress:
		if isCoachingRequest(userMessage) {
			state.CoachingRequested = true
			state.ActiveMode = modeCoaching
			state.CoachingEnteredBeforeResults = true
		}
	case phaseAssessmentResults:
		if isCoachingRequest(userMessage) {
			state.CoachingRequested = true
			state.ActiveMode = modeCoaching
		}
	}
}

func applyBootstrapState(state *AgentSessionState, assistant string) {
	state.StartupPromptShown = true
	state.FirstUserMessageSeen = true
	state.ConversationPhase = phaseAwaitingMajor
	state.WaitingForUserResponse = true
	state.PendingQuestion = "studentMajor"
	state.MessageIndex = 1
	state.LastAssistantSummary = truncateSummary(assistant, 500)
}

func parseAssistantStateSync(state *AgentSessionState, assistant string) {
	if block := ipyintervuBlockPattern.FindStringSubmatch(assistant); len(block) == 2 {
		applyIPyIntervuTailJSON(state, block[1])
	} else if partial := partialIPyIntervuOpenPattern.FindStringSubmatch(assistant); len(partial) == 2 {
		applyIPyIntervuTailJSON(state, partial[1])
	}

	for field, pattern := range bucketPatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		switch field {
		case "conceptualAssessmentBucket":
			state.ConceptualAssessmentBucket = normalizeAssessmentBucket(value)
		case "codeAssessmentBucket":
			state.CodeAssessmentBucket = normalizeAssessmentBucket(value)
		case "bugAssessmentBucket":
			state.BugAssessmentBucket = normalizeAssessmentBucket(value)
		case "businessDomain":
			state.BusinessDomain = value
		}
	}

	for field, pattern := range jsonBucketPatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		switch field {
		case "conceptualAssessmentBucket":
			state.ConceptualAssessmentBucket = normalizeAssessmentBucket(value)
		case "codeAssessmentBucket":
			state.CodeAssessmentBucket = normalizeAssessmentBucket(value)
		case "bugAssessmentBucket":
			state.BugAssessmentBucket = normalizeAssessmentBucket(value)
		}
	}

	detectProseAssessmentBuckets(state, assistant)
}

func applyIPyIntervuTailJSON(state *AgentSessionState, rawJSON string) {
	jsonText := strings.TrimSpace(rawJSON)
	jsonText = strings.TrimSuffix(jsonText, "```")
	jsonText = strings.TrimSpace(jsonText)
	if jsonText == "" {
		return
	}

	var tail ipyintervuTail
	if err := json.Unmarshal([]byte(jsonText), &tail); err == nil {
		if tail.ConceptualAssessmentBucket != "" {
			state.ConceptualAssessmentBucket = normalizeAssessmentBucket(tail.ConceptualAssessmentBucket)
		}
		if tail.CodeAssessmentBucket != "" {
			state.CodeAssessmentBucket = normalizeAssessmentBucket(tail.CodeAssessmentBucket)
		}
		if tail.BugAssessmentBucket != "" {
			state.BugAssessmentBucket = normalizeAssessmentBucket(tail.BugAssessmentBucket)
		}
		if tail.BusinessDomain != nil {
			if raw, err := json.Marshal(tail.BusinessDomain); err == nil {
				state.BusinessDomain = string(raw)
			}
		}
		return
	}

	for field, pattern := range jsonBucketPatterns {
		match := pattern.FindStringSubmatch(jsonText)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		switch field {
		case "conceptualAssessmentBucket":
			state.ConceptualAssessmentBucket = normalizeAssessmentBucket(value)
		case "codeAssessmentBucket":
			state.CodeAssessmentBucket = normalizeAssessmentBucket(value)
		case "bugAssessmentBucket":
			state.BugAssessmentBucket = normalizeAssessmentBucket(value)
		}
	}
}

func normalizeAssessmentBucket(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "not yet ready", "not ready yet", "weak":
		return bucketNotReady
	case "looking good", "competent", "strong", "good", "solid":
		return bucketCompetent
	case "looks great", "exceptional", "excellent", "outstanding":
		return bucketExceptional
	case "n/a", "na":
		return bucketNA
	default:
		return strings.TrimSpace(value)
	}
}

func firstProseBucketMatch(assistant string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) >= 2 {
			return normalizeAssessmentBucket(match[1])
		}
	}
	return ""
}

func detectProseAssessmentBuckets(state *AgentSessionState, assistant string) {
	if state.ConceptualAssessmentBucket == "" && state.ActiveMode == modeConceptual {
		if bucket := firstProseBucketMatch(assistant, proseConceptualBucketPatterns); bucket != "" {
			state.ConceptualAssessmentBucket = bucket
		}
	}
	if state.CodeAssessmentBucket == "" && state.ActiveMode == modeCode {
		if bucket := firstProseBucketMatch(assistant, proseCodeBucketPatterns); bucket != "" {
			state.CodeAssessmentBucket = bucket
		}
	}
	if state.BugAssessmentBucket == "" && state.ActiveMode == modeBug {
		if bucket := firstProseBucketMatch(assistant, proseBugBucketPatterns); bucket != "" {
			state.BugAssessmentBucket = bucket
		}
	}
}

func appendModeCompleted(state *AgentSessionState, mode string) {
	for _, existing := range state.ModesCompleted {
		if existing == mode {
			return
		}
	}
	state.ModesCompleted = append(state.ModesCompleted, mode)
}

func computeFinalRating(state *AgentSessionState) string {
	if state.isProblemDecompositionWeek() {
		if state.ConceptualAssessmentBucket != "" {
			return state.ConceptualAssessmentBucket
		}
		return ""
	}

	buckets := []string{
		state.ConceptualAssessmentBucket,
		state.CodeAssessmentBucket,
		state.BugAssessmentBucket,
	}
	for _, bucket := range buckets {
		if bucket == bucketNotReady {
			return bucketNotReady
		}
	}
	hasCompetent := false
	allExceptional := true
	for _, bucket := range buckets {
		if bucket == "" || bucket == bucketNA {
			continue
		}
		if bucket == bucketCompetent {
			hasCompetent = true
			allExceptional = false
		}
		if bucket != bucketExceptional {
			allExceptional = false
		}
	}
	if hasCompetent {
		return bucketCompetent
	}
	if allExceptional {
		return bucketExceptional
	}
	return ""
}

func markAssessmentStarted(state *AgentSessionState) {
	now := time.Now()
	state.AssessmentStartTime = &now
	state.AssessmentEndTime = nil
	state.AssessmentComplete = false
}

func markAssessmentEnded(state *AgentSessionState) {
	if state.AssessmentEndTime != nil {
		return
	}
	now := time.Now()
	state.AssessmentEndTime = &now
}

// applyAutomaticModeTransitions updates activeMode after bucket assignment.
// Returns true when the server should immediately continue into the next assessment mode.
func applyAutomaticModeTransitions(state *AgentSessionState) bool {
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return false
	}

	if state.ConversationPhase == phaseAssessmentResults {
		return false
	}

	if state.isProblemDecompositionWeek() {
		if state.ConceptualAssessmentBucket != "" && state.ConversationPhase != phaseAssessmentResults {
			appendModeCompleted(state, modeConceptual)
			state.CodeAssessmentBucket = bucketNA
			state.BugAssessmentBucket = bucketNA
			state.ActiveMode = ""
			state.ConversationPhase = phaseAssessmentResults
			state.FinalRating = computeFinalRating(state)
			state.AssessmentComplete = state.FinalRating != ""
			state.PendingQuestion = "assessmentResults"
			state.WaitingForUserResponse = true
			state.CoachingEnteredBeforeResults = false
			markAssessmentEnded(state)
		}
		return false
	}

	switch state.ActiveMode {
	case modeConceptual:
		if state.ConceptualAssessmentBucket != "" {
			appendModeCompleted(state, modeConceptual)
			state.ActiveMode = modeCode
			state.PendingQuestion = "codeAssessment"
			state.WaitingForUserResponse = true
			return true
		}
	case modeCode:
		if state.CodeAssessmentBucket != "" {
			appendModeCompleted(state, modeCode)
			state.ActiveMode = modeBug
			state.PendingQuestion = "bugAssessment"
			state.WaitingForUserResponse = true
			return true
		}
	case modeBug:
		if state.BugAssessmentBucket != "" {
			appendModeCompleted(state, modeBug)
			state.ActiveMode = ""
			state.ConversationPhase = phaseAssessmentResults
			state.FinalRating = computeFinalRating(state)
			state.AssessmentComplete = state.FinalRating != ""
			state.PendingQuestion = "assessmentResults"
			state.WaitingForUserResponse = true
			state.CoachingEnteredBeforeResults = false
			markAssessmentEnded(state)
		}
	}
	return false
}

func modeContinuationUserMessage(state *AgentSessionState) string {
	switch state.ActiveMode {
	case modeCode:
		return "[System handoff: Server state activeMode is now CodeProblem. Begin the code assessment immediately: Taylor and Morgan introduce themselves at the company from businessDomain, then present the first step of the coding interview (decomposition before code). Do not ask the student to choose or confirm the next mode.]"
	case modeBug:
		return "[System handoff: Server state activeMode is now BugHunting. Begin the bug-hunting assessment immediately with interviewer introductions and the first bug-finding step. Do not ask the student to choose or confirm the next mode.]"
	default:
		return ""
	}
}

func applyPostChatStateUpdate(state *AgentSessionState, userMessage, assistant string) bool {
	parseAssistantStateSync(state, assistant)
	shouldContinue := applyAutomaticModeTransitions(state)
	state.LastAssistantSummary = truncateSummary(assistant, 500)
	return shouldContinue
}
