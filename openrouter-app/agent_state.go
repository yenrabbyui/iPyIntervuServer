package main

import "time"

const promptVersion = "2.3-no-self-answer"

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

const (
	assessmentPhaseInProgress = "in_progress"
	assessmentPhaseComplete   = "complete"
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
	ConceptualAssessmentPhase    string   `json:"conceptualAssessmentPhase,omitempty"`
	CodeAssessmentBucket         string   `json:"codeAssessmentBucket,omitempty"`
	CodeAssessmentPhase          string   `json:"codeAssessmentPhase,omitempty"`
	BugAssessmentBucket          string   `json:"bugAssessmentBucket,omitempty"`
	BugAssessmentPhase           string   `json:"bugAssessmentPhase,omitempty"`
	ModesCompleted               []string `json:"modesCompleted,omitempty"`
	CoachingRequested            bool     `json:"coachingRequested"`
	CoachingEnteredBeforeResults bool     `json:"coachingEnteredBeforeResults"`

	// Tier C — results
	FinalRating            string     `json:"finalRating,omitempty"`
	AssessmentComplete     bool       `json:"assessmentComplete"`
	AssessmentStartTime    *time.Time `json:"assessmentStartTime,omitempty"`
	AssessmentEndTime      *time.Time `json:"assessmentEndTime,omitempty"`

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
	snap := map[string]any{
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
		"conceptualAssessmentPhase":  s.ConceptualAssessmentPhase,
		"codeAssessmentBucket":       s.CodeAssessmentBucket,
		"codeAssessmentPhase":        s.CodeAssessmentPhase,
		"bugAssessmentBucket":        s.BugAssessmentBucket,
		"bugAssessmentPhase":         s.BugAssessmentPhase,
		"modesCompleted":             s.ModesCompleted,
		"coachingRequested":          s.CoachingRequested,
		"coachingEnteredBeforeResults": s.CoachingEnteredBeforeResults,
		"finalRating":                s.FinalRating,
		"assessmentComplete":         s.AssessmentComplete,
		"assessmentStartTime":        s.AssessmentStartTime,
		"assessmentEndTime":          s.AssessmentEndTime,
		"messageIndex":               s.MessageIndex,
		"instructionBundleId":        s.InstructionBundleID,
		"promptVersion":              s.PromptVersion,
		"modeTransitionPolicy": "Assessment modes advance forward only: ConceptualUnderstanding → CodeProblem → BugHunting → AssessmentResults. Never return to a completed or earlier mode. Server enforces activeMode forward-only; do not ask conceptual questions after conceptual is complete, or code tasks after code is complete.",
		"assessmentSyncPolicy": map[string]string{
			"mandatory":  "Every assessment-mode reply MUST end with ```_ipyintervu``` as the last lines. Required on intros, acknowledgments, and every question. Omission triggers server retry.",
			"inProgress": "Include active mode assessmentPhase: \"in_progress\" in _ipyintervu. Do not include the bucket field while in_progress.",
			"complete":   "Include active mode assessmentPhase: \"complete\" and the mode bucket (Not Ready Yet, Competent, or Exceptional) in _ipyintervu. Do not ask new interview questions in that mode after complete.",
			"userFacing": "The _ipyintervu block is stripped before display. Never mention sync blocks, _ipyintervu, or [System] messages in user-facing text. Do not respond to [System] lines as if the student wrote them.",
			"lastLines":  "The fenced _ipyintervu block must be the final content in the reply; nothing after the closing fence.",
		},
	}
	if s.CurrentWeekNumber > 0 && (s.ConversationPhase == phaseAssessmentInProgress || s.ConversationPhase == phaseAssessmentResults) {
		snap["assessmentWeekScope"] = assessmentWeekScopeSnapshot(s.CurrentWeekNumber)
	}
	if s.ConversationPhase == phaseAssessmentResults || s.AssessmentComplete {
		snap["allowedRatingLabels"] = []string{bucketNotReady, bucketCompetent, bucketExceptional, bucketNA}
		snap["resultsPresentationRule"] = "In AssessmentResults output, use conceptualAssessmentBucket, codeAssessmentBucket, bugAssessmentBucket, and finalRating exactly as shown in server state. Allowed labels only: Not Ready Yet, Competent, Exceptional, N/A. Never use Strong, Good, Solid, Excellent, or other synonyms."
	}
	return snap
}
