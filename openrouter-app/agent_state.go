package main

import "time"

const promptVersion = "2.0-fragments"

const (
	phaseAwaitingMajor              = "AwaitingMajor"
	phaseAwaitingKeyConcept         = "AwaitingKeyConceptSelection"
	phaseAssessmentInProgress       = "AssessmentInProgress"
	phaseAssessmentResults          = "AssessmentResults"
)

const (
	modeConceptual = "ConceptualUnderstanding"
	modeCode       = "CodeProblem"
	modeBug        = "BugHunting"
	modeCoaching   = "Coaching"
)

const (
	bucketNotReady    = "Not Ready Yet"
	bucketCompetent   = "Competent"
	bucketExceptional = "Exceptional"
	bucketNA          = "N/A"
)

// AgentSessionState is the server-owned session state (Tiers A–D).
type AgentSessionState struct {
	// Tier A — setup and conversation control
	ConversationPhase      string `json:"conversationPhase"`
	StartupPromptShown     bool   `json:"startupPromptShown"`
	FirstUserMessageSeen   bool   `json:"firstUserMessageSeen"`
	StudentMajor           string `json:"studentMajor,omitempty"`
	SelectedKeyConcept     string `json:"selectedKeyConcept,omitempty"`
	CurrentWeekNumber      int    `json:"currentWeekNumber,omitempty"`
	WaitingForUserResponse bool   `json:"waitingForUserResponse"`
	PendingQuestion        string `json:"pendingQuestion,omitempty"`
	InstructionBundleID    string `json:"instructionBundleId"`
	PromptVersion          string `json:"promptVersion"`

	// Tier B — assessment runtime
	ActiveMode                   string   `json:"activeMode,omitempty"`
	BusinessDomain               string   `json:"businessDomain,omitempty"`
	ConceptualAssessmentBucket   string   `json:"conceptualAssessmentBucket,omitempty"`
	CodeAssessmentBucket         string   `json:"codeAssessmentBucket,omitempty"`
	BugAssessmentBucket          string   `json:"bugAssessmentBucket,omitempty"`
	ModesCompleted               []string `json:"modesCompleted,omitempty"`
	CoachingRequested            bool     `json:"coachingRequested"`

	// Tier C — results
	FinalRating          string `json:"finalRating,omitempty"`
	AssessmentComplete   bool   `json:"assessmentComplete"`

	// Tier D — operational tracking
	MessageIndex         int      `json:"messageIndex"`
	LastUserMessageRaw   string   `json:"lastUserMessageRaw,omitempty"`
	LastAssistantSummary string   `json:"lastAssistantSummary,omitempty"`
	KBFilesLoaded        []string `json:"kbFilesLoaded"`

	UpdatedAt time.Time `json:"-"`
}

func newAgentSessionState() *AgentSessionState {
	return &AgentSessionState{
		ConversationPhase: phaseAwaitingMajor,
		PromptVersion:     promptVersion,
		WaitingForUserResponse: true,
		PendingQuestion:   "studentMajor",
	}
}

func (s *AgentSessionState) isProblemDecompositionWeek() bool {
	return s.CurrentWeekNumber == 1 ||
		s.SelectedKeyConcept == "Week 1 - Problem Decomposition"
}

func (s *AgentSessionState) snapshotForPrompt() map[string]any {
	return map[string]any{
		"conversationPhase":          s.ConversationPhase,
		"startupPromptShown":         s.StartupPromptShown,
		"firstUserMessageSeen":       s.FirstUserMessageSeen,
		"studentMajor":               s.StudentMajor,
		"selectedKeyConcept":         s.SelectedKeyConcept,
		"currentWeekNumber":          s.CurrentWeekNumber,
		"waitingForUserResponse":     s.WaitingForUserResponse,
		"pendingQuestion":            s.PendingQuestion,
		"activeMode":                 s.ActiveMode,
		"businessDomain":             s.BusinessDomain,
		"conceptualAssessmentBucket": s.ConceptualAssessmentBucket,
		"codeAssessmentBucket":       s.CodeAssessmentBucket,
		"bugAssessmentBucket":        s.BugAssessmentBucket,
		"modesCompleted":             s.ModesCompleted,
		"coachingRequested":          s.CoachingRequested,
		"finalRating":                s.FinalRating,
		"assessmentComplete":         s.AssessmentComplete,
		"messageIndex":               s.MessageIndex,
		"instructionBundleId":        s.InstructionBundleID,
		"promptVersion":              s.PromptVersion,
		"modeTransitionPolicy":       "Server enforces automatic mode transitions after bucket assignment; do not ask the user to choose the next assessment mode.",
	}
}
