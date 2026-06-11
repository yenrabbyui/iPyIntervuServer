package main

import "testing"

func TestNormalizeAssessmentBucket(t *testing.T) {
	tests := map[string]string{
		"Looking Good":  bucketCompetent,
		"Not Yet Ready": bucketNotReady,
		"Looks Great":   bucketExceptional,
		"Competent":     bucketCompetent,
		"Strong":        bucketCompetent,
		"Good":          bucketCompetent,
	}
	for input, want := range tests {
		if got := normalizeAssessmentBucket(input); got != want {
			t.Fatalf("normalizeAssessmentBucket(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseAssistantStateSyncConceptualBucketFromProse(t *testing.T) {
	state := &AgentSessionState{ActiveMode: modeConceptual}
	assistant := "Thanks for walking through those concepts. Your conceptual assessment is Competent."

	parseAssistantStateSync(state, assistant)

	if state.ConceptualAssessmentBucket != bucketCompetent {
		t.Fatalf("ConceptualAssessmentBucket = %q, want %q", state.ConceptualAssessmentBucket, bucketCompetent)
	}
}

func TestParseAssistantStateSyncPartialIPyJSONBucket(t *testing.T) {
	state := &AgentSessionState{ActiveMode: modeConceptual}
	assistant := "I think we have a solid picture.\n\n```_ipy\n{\"conceptualAssessmentBucket\": \"Competent\"}"

	parseAssistantStateSync(state, assistant)

	if state.ConceptualAssessmentBucket != bucketCompetent {
		t.Fatalf("ConceptualAssessmentBucket = %q, want %q", state.ConceptualAssessmentBucket, bucketCompetent)
	}
}

func TestApplyPostChatStateUpdatePartialIPyTriggersCodeMode(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 8,
	}
	assistant := "Let me wrap up this portion.\n\n```_ipy\n{\"conceptualAssessmentBucket\": \"Competent\"}"

	if !applyPostChatStateUpdate(state, "student answer", assistant) {
		t.Fatal("expected automatic continuation into code mode")
	}
	if state.ActiveMode != modeCode {
		t.Fatalf("ActiveMode = %q, want %q", state.ActiveMode, modeCode)
	}
}

func TestApplyAutomaticModeTransitionsConceptualToCode(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:          phaseAssessmentInProgress,
		ActiveMode:                 modeConceptual,
		CurrentWeekNumber:          8,
		ConceptualAssessmentBucket: bucketCompetent,
	}

	if !applyAutomaticModeTransitions(state) {
		t.Fatal("expected continuation after conceptual bucket assignment")
	}
	if state.ActiveMode != modeCode {
		t.Fatalf("ActiveMode = %q, want %q", state.ActiveMode, modeCode)
	}
	if state.PendingQuestion != "codeAssessment" {
		t.Fatalf("PendingQuestion = %q, want codeAssessment", state.PendingQuestion)
	}
}

func TestCoachingEnteredBeforeResultsClearedOnFinalResults(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:            phaseAssessmentInProgress,
		ActiveMode:                   modeBug,
		CurrentWeekNumber:            8,
		ConceptualAssessmentBucket:   bucketCompetent,
		CodeAssessmentBucket:         bucketCompetent,
		BugAssessmentBucket:          bucketCompetent,
		CoachingEnteredBeforeResults: true,
	}

	if applyAutomaticModeTransitions(state) {
		t.Fatal("expected no continuation after final bug bucket assignment")
	}
	if state.ConversationPhase != phaseAssessmentResults {
		t.Fatalf("ConversationPhase = %q, want %q", state.ConversationPhase, phaseAssessmentResults)
	}
	if state.CoachingEnteredBeforeResults {
		t.Fatal("CoachingEnteredBeforeResults should clear once final results are reached")
	}
}

func TestModeContinuationUserMessage(t *testing.T) {
	state := &AgentSessionState{ActiveMode: modeCode}
	msg := modeContinuationUserMessage(state)
	if msg == "" {
		t.Fatal("expected handoff message for code mode")
	}
}
