package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	bucketPatterns = map[string]*regexp.Regexp{
		"conceptualAssessmentBucket": regexp.MustCompile(`(?i)conceptualAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional)`),
		"codeAssessmentBucket":       regexp.MustCompile(`(?i)codeAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional|N/A)`),
		"bugAssessmentBucket":        regexp.MustCompile(`(?i)bugAssessmentBucket\s*[=:]\s*(Not Ready Yet|Competent|Exceptional|N/A)`),
		"businessDomain":             regexp.MustCompile(`(?i)businessDomain\s*[=:]\s*(.+)`),
	}
	ipyintervuBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipyintervu\\s*\\n(.*?)\\n```")
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
		}
	case phaseAssessmentInProgress, phaseAssessmentResults:
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
		var tail ipyintervuTail
		if err := json.Unmarshal([]byte(strings.TrimSpace(block[1])), &tail); err == nil {
			if tail.ConceptualAssessmentBucket != "" {
				state.ConceptualAssessmentBucket = tail.ConceptualAssessmentBucket
			}
			if tail.CodeAssessmentBucket != "" {
				state.CodeAssessmentBucket = tail.CodeAssessmentBucket
			}
			if tail.BugAssessmentBucket != "" {
				state.BugAssessmentBucket = tail.BugAssessmentBucket
			}
			if tail.BusinessDomain != nil {
				if raw, err := json.Marshal(tail.BusinessDomain); err == nil {
					state.BusinessDomain = string(raw)
				}
			}
		}
	}

	for field, pattern := range bucketPatterns {
		match := pattern.FindStringSubmatch(assistant)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		switch field {
		case "conceptualAssessmentBucket":
			state.ConceptualAssessmentBucket = value
		case "codeAssessmentBucket":
			state.CodeAssessmentBucket = value
		case "bugAssessmentBucket":
			state.BugAssessmentBucket = value
		case "businessDomain":
			state.BusinessDomain = value
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

func applyAutomaticModeTransitions(state *AgentSessionState) {
	if state.CoachingRequested && state.ActiveMode == modeCoaching {
		return
	}

	if state.ConversationPhase == phaseAssessmentResults {
		return
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
		}
		return
	}

	switch state.ActiveMode {
	case modeConceptual:
		if state.ConceptualAssessmentBucket != "" {
			appendModeCompleted(state, modeConceptual)
			state.ActiveMode = modeCode
			state.PendingQuestion = "codeAssessment"
			state.WaitingForUserResponse = true
		}
	case modeCode:
		if state.CodeAssessmentBucket != "" {
			appendModeCompleted(state, modeCode)
			state.ActiveMode = modeBug
			state.PendingQuestion = "bugAssessment"
			state.WaitingForUserResponse = true
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
		}
	}
}

func applyPostChatStateUpdate(state *AgentSessionState, userMessage, assistant string) {
	parseAssistantStateSync(state, assistant)
	applyAutomaticModeTransitions(state)
	state.LastAssistantSummary = truncateSummary(assistant, 500)
}
