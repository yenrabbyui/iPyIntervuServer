package main

import (
	"strings"
	"testing"
)

func TestIsFollowUpAssessmentTurn(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeBug,
	}
	if isFollowUpAssessmentTurn(state) {
		t.Fatal("expected false before opening served")
	}

	state.ModeOpeningServed = true
	if isFollowUpAssessmentTurn(state) {
		t.Fatal("expected false before user answered")
	}

	state.ModeUserAnsweredSinceOpening = true
	if !isFollowUpAssessmentTurn(state) {
		t.Fatal("expected follow-up turn after user answered opening")
	}
}

func TestForwardMissingSyncHandoffOnFollowUp(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:            phaseAssessmentInProgress,
		ActiveMode:                   modeBug,
		ModeInterviewStep:            interviewStepFollowUp,
		ModeOpeningServed:            true,
		ModeUserAnsweredSinceOpening: true,
	}
	handoff := buildUnifiedCorrectiveHandoff(state, assessmentViolations{MissingSync: true})
	if strings.Contains(handoff, "Re-present the SAME concrete scenario") {
		t.Fatalf("follow-up missing sync should forward, not rewind: %q", handoff)
	}
	if !strings.Contains(handoff, "do NOT re-present the opening scenario") {
		t.Fatalf("expected forward guidance, got %q", handoff)
	}
	if !strings.Contains(handoff, "debugging-process follow-up") {
		t.Fatalf("expected bug follow-up guidance, got %q", handoff)
	}
}

func TestRewindMissingSyncHandoffOnOpeningTurn(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeBug,
		ModeInterviewStep: interviewStepOpening,
	}
	handoff := buildUnifiedCorrectiveHandoff(state, assessmentViolations{MissingSync: true})
	if !strings.Contains(handoff, "Re-present the SAME concrete scenario") {
		t.Fatalf("opening missing sync should rewind, got %q", handoff)
	}
}

func TestContentViolationUsesRewindEvenOnFollowUp(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:            phaseAssessmentInProgress,
		ActiveMode:                   modeCode,
		ModeOpeningServed:            true,
		ModeUserAnsweredSinceOpening: true,
	}
	handoff := buildUnifiedCorrectiveHandoff(state, assessmentViolations{Content: true, MissingSync: true})
	if !strings.Contains(handoff, "Re-present the SAME concrete scenario") {
		t.Fatalf("content violation should rewind even on follow-up, got %q", handoff)
	}
}

func TestAdvanceInterviewProgressOnUserAnswer(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeCode,
		ModeInterviewStep: interviewStepAwaitingAnswer,
		ModeOpeningServed: true,
	}
	advanceInterviewProgressOnUserAnswer(state)
	if !state.ModeUserAnsweredSinceOpening {
		t.Fatal("expected userAnsweredSinceOpening")
	}
	if state.ModeInterviewStep != interviewStepDecompositionAnswered {
		t.Fatalf("step = %q, want %q", state.ModeInterviewStep, interviewStepDecompositionAnswered)
	}
}

func TestUpdateInterviewProgressAfterAssistantOpening(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeBug,
	}
	assistant := "What would you check first?\n\n```_ipyintervu\n{\"bugAssessmentPhase\": \"in_progress\"}\n```"
	updateInterviewProgressAfterAssistant(state, assistant)
	if !state.ModeOpeningServed {
		t.Fatal("expected opening served")
	}
	if state.ModeInterviewStep != interviewStepAwaitingAnswer {
		t.Fatalf("step = %q, want %q", state.ModeInterviewStep, interviewStepAwaitingAnswer)
	}
}

func TestCodeStepAdvancesToAwaitingCodeFromAssistant(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:            phaseAssessmentInProgress,
		ActiveMode:                   modeCode,
		ModeInterviewStep:            interviewStepDecompositionAnswered,
		ModeOpeningServed:            true,
		ModeUserAnsweredSinceOpening: true,
	}
	assistant := "Thanks. Please paste your Python code here.\n\n```_ipyintervu\n{\"codeAssessmentPhase\": \"in_progress\"}\n```"
	updateInterviewProgressAfterAssistant(state, assistant)
	if state.ModeInterviewStep != interviewStepAwaitingCode {
		t.Fatalf("step = %q, want %q", state.ModeInterviewStep, interviewStepAwaitingCode)
	}
}

func TestResetModeInterviewProgressOnModeTransition(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase:            phaseAssessmentInProgress,
		ActiveMode:                   modeConceptual,
		ConceptualAssessmentPhase:      assessmentPhaseComplete,
		ConceptualAssessmentBucket:   bucketCompetent,
		ModeInterviewStep:            interviewStepInterviewing,
		ModeOpeningServed:            true,
		ModeUserAnsweredSinceOpening: true,
	}
	if !applyAutomaticModeTransitions(state) {
		t.Fatal("expected mode continuation into code")
	}
	if state.ActiveMode != modeCode {
		t.Fatalf("activeMode = %q, want %q", state.ActiveMode, modeCode)
	}
	if state.ModeInterviewStep != interviewStepOpening || state.ModeOpeningServed {
		t.Fatalf("expected reset progress, got step=%q openingServed=%v", state.ModeInterviewStep, state.ModeOpeningServed)
	}
}

func TestInterviewProgressSnapshotIncludedForAssessment(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeCode,
		ModeInterviewStep: interviewStepDecompositionAnswered,
		ModeOpeningServed: true,
	}
	snap := state.snapshotForPrompt()
	progress, ok := snap["interviewProgress"].(map[string]any)
	if !ok {
		t.Fatalf("expected interviewProgress in snapshot, got %#v", snap["interviewProgress"])
	}
	if progress["step"] != interviewStepDecompositionAnswered {
		t.Fatalf("step = %v", progress["step"])
	}
}

func TestPostProcessUpdatesProgressOnSuccess(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 8,
	}
	assistant := "Got it. What would happen if the list were empty?\n\n```_ipyintervu\n{\"conceptualAssessmentPhase\": \"in_progress\"}\n```"
	postProcessAssistantTurnWithGuard(state, assistant, false, nil)
	if !state.ModeOpeningServed {
		t.Fatal("expected progress updated on successful turn")
	}
}

func TestPostProcessDoesNotUpdateProgressBeforeCorrectiveRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeBug,
	}
	assistant := "Got it."
	followUp := postProcessAssistantTurnWithGuard(state, assistant, false, nil)
	if followUp.Kind != "corrective_retry" {
		t.Fatalf("expected corrective_retry, got %+v", followUp)
	}
	if state.ModeOpeningServed {
		t.Fatal("should not mark opening served before corrective retry succeeds")
	}
}
