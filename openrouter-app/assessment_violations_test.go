package main

import (
	"strings"
	"testing"
)

func TestDetectAssessmentViolationsMissingSync(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 8,
	}
	assistant := "What would you do if the user enters an invalid menu choice?"

	v := detectAssessmentViolations(state, assistant)
	if !v.MissingSync {
		t.Fatal("expected missing sync violation")
	}
	if !v.NeedsCorrectiveRetry() {
		t.Fatal("expected corrective retry for missing sync")
	}
}

func TestDetectAssessmentViolationsContentWithSyncTruncateOnly(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 9,
	}
	assistant := "What is append? Append adds a single element to the end of a list.\n\n```_ipyintervu\n{\"conceptualAssessmentPhase\": \"in_progress\"}\n```"

	v := detectAssessmentViolations(state, assistant)
	if !v.Content {
		t.Fatal("expected content violation")
	}
	if v.MissingSync {
		t.Fatal("sync block is present")
	}
	if !v.NeedsTruncateOnly() {
		t.Fatal("expected truncate-only handling")
	}
	if v.NeedsCorrectiveRetry() {
		t.Fatal("content with valid sync should not retry")
	}
}

func TestPostProcessUnifiedCorrectiveRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 8,
	}
	assistant := "What would you do if the user enters an invalid menu choice?"

	followUp := postProcessAssistantTurn(state, assistant, false, nil)
	if followUp.Kind != "corrective_retry" || !followUp.ContinueTurn {
		t.Fatalf("expected corrective_retry, got %+v", followUp)
	}
	if !strings.Contains(followUp.Handoff, "_ipyintervu") {
		t.Fatalf("expected sync requirement in handoff, got %q", followUp.Handoff)
	}
}

func TestPostProcessContentWithSyncNoRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 9,
	}
	assistant := "What is append? Append adds a single element to the end of a list.\n\n```_ipyintervu\n{\"conceptualAssessmentPhase\": \"in_progress\"}\n```"

	followUp := postProcessAssistantTurn(state, assistant, false, nil)
	if followUp.ContinueTurn {
		t.Fatalf("expected no retry for truncate-only content issue, got %+v", followUp)
	}
}

func TestPostProcessSevereCompositeWithSyncCorrectiveRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 2,
	}
	assistant := strings.Join([]string{
		"Got it. At QuantCore, we often work with exact integer results. If you had meters = 7 and pieces = 2, what would the expression parts = meters // pieces produce, and what data type would it be?",
		"the type would be integer. The value would be 3.",
		"Got it. After that runs, y would still be 7 because Python evaluated y = x + 2 using the value of x at that moment (5), and reassigning x later does not retroactively update y.",
		"",
		"Now, at QuantCore we often work with area calculations. If a rectangle has width = 4 and height = 7, what expression would you write to compute its area, and what data type would the result be?",
		"",
		"```_ipyintervu",
		"{\"conceptualAssessmentPhase\": \"in_progress\"}",
		"```",
	}, "\n")

	v := detectAssessmentViolations(state, assistant)
	if !v.SevereContent {
		t.Fatal("expected severe composite violation")
	}
	if !v.NeedsCorrectiveRetry() {
		t.Fatal("expected corrective retry for severe composite even with sync")
	}

	followUp := postProcessAssistantTurn(state, assistant, false, nil)
	if followUp.Kind != "corrective_retry" || !followUp.ContinueTurn {
		t.Fatalf("expected corrective_retry, got %+v", followUp)
	}
}

func TestPostProcessSimulatedStudentLineCorrectiveRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 1,
	}
	assistant := strings.Join([]string{
		"What would you consider the final output of this process to be?",
		"a table where each row was a region and an amount",
		"Good point — how might you handle invalid region codes?",
	}, "\n")

	followUp := postProcessAssistantTurn(state, assistant, false, nil)
	if followUp.Kind != "corrective_retry" || !followUp.ContinueTurn {
		t.Fatalf("expected corrective_retry, got %+v", followUp)
	}
}

func TestPostProcessFailClosedAfterCorrectiveRetry(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 8,
	}
	assistant := "What would you do if the user enters an invalid menu choice?"

	followUp := postProcessAssistantTurn(state, assistant, true, nil)
	if followUp.Kind != "fail_closed" {
		t.Fatalf("expected fail_closed after corrective retry, got %+v", followUp)
	}
	if followUp.DirectAssistant == "" {
		t.Fatal("expected direct failure message")
	}
}

func TestWeek1CompleteBucketUsesServerResults(t *testing.T) {
	state := &AgentSessionState{
		ConversationPhase: phaseAssessmentInProgress,
		ActiveMode:        modeConceptual,
		CurrentWeekNumber: 1,
		SelectedKeyConcept: "Week 1 - Problem Decomposition",
	}
	assistant := "Understood.\n\n```_ipyintervu\n{\"conceptualAssessmentPhase\": \"complete\", \"conceptualAssessmentBucket\": \"Competent\"}\n```"

	followUp := postProcessAssistantTurn(state, assistant, false, nil)
	if followUp.Kind != "server_results" {
		t.Fatalf("expected server_results, got %+v", followUp)
	}
	if followUp.ContinueTurn {
		t.Fatal("server results should not continue model turn")
	}
	if !strings.Contains(followUp.DirectAssistant, "Overall Rating: Competent") {
		t.Fatalf("expected server-rendered results, got %q", followUp.DirectAssistant)
	}
	if state.ConversationPhase != phaseAssessmentResults {
		t.Fatalf("ConversationPhase = %q, want %q", state.ConversationPhase, phaseAssessmentResults)
	}
}

func TestBuildServerAssessmentResultsMessageWeek1(t *testing.T) {
	state := &AgentSessionState{
		SelectedKeyConcept:         "Week 1 - Problem Decomposition",
		ConceptualAssessmentBucket: bucketCompetent,
		FinalRating:                bucketCompetent,
	}
	state.CurrentWeekNumber = 1

	got := buildServerAssessmentResultsMessage(state)
	if !strings.Contains(got, "Code Assessment: N/A") {
		t.Fatalf("expected N/A code bucket, got %q", got)
	}
	if !strings.Contains(got, "Overall Rating: Competent") {
		t.Fatalf("expected final rating, got %q", got)
	}
}
