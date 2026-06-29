package main

import "strings"

const (
	interviewStepOpening                 = "opening"
	interviewStepAwaitingAnswer          = "awaiting_answer"
	interviewStepInterviewing            = "interviewing"
	interviewStepDecompositionAnswered   = "decomposition_answered"
	interviewStepAwaitingCode            = "awaiting_code"
	interviewStepCodeSubmitted           = "code_submitted"
	interviewStepFollowUp                = "follow_up"
)

func resetModeInterviewProgress(state *AgentSessionState) {
	if state == nil {
		return
	}
	state.ModeInterviewStep = interviewStepOpening
	state.ModeOpeningServed = false
	state.ModeUserAnsweredSinceOpening = false
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
			}
		}
	case modeBug:
		if state.ModeInterviewStep == interviewStepOpening || state.ModeInterviewStep == interviewStepAwaitingAnswer {
			state.ModeInterviewStep = interviewStepFollowUp
		}
	}
}

func updateInterviewProgressAfterAssistant(state *AgentSessionState, assistant string) {
	if state == nil || state.ConversationPhase != phaseAssessmentInProgress || !hasIPyIntervuBlock(assistant) {
		return
	}
	switch state.ActiveMode {
	case modeConceptual, modeCode, modeBug:
		if !state.ModeOpeningServed {
			state.ModeOpeningServed = true
			state.ModeInterviewStep = interviewStepAwaitingAnswer
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
	return false
}

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
			return "The student submitted code. Ask one explain-code, line-level, or AI-use reflection question — do not repeat decomposition or re-request the same code."
		default:
			return "Continue the code interview with the next step (code request or explain-code). Do not repeat the opening decomposition question."
		}
	case modeBug:
		if state.ModeInterviewStep == interviewStepFollowUp || state.ModeInterviewStep == interviewStepInterviewing {
			return "Continue bug hunting with one new debugging-process follow-up question. Do not repeat the opening scenario question verbatim."
		}
		return "Continue bug hunting with one debugging-process question. Do not repeat the opening scenario question verbatim."
	case modeConceptual:
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
	switch state.ActiveMode {
	case modeCode:
		snap["forwardPolicy"] = "After decomposition is answered, explicitly ask the student to paste their Python code — then evaluate pasted code before complete. Never repeat the opening decomposition ask or complete after decomposition alone."
	case modeBug:
		snap["forwardPolicy"] = "After the opening debug question is answered, ask follow-up debugging-process questions — never repeat the opening question verbatim."
	case modeConceptual:
		snap["forwardPolicy"] = "Ask new conceptual follow-ups; do not repeat the same question verbatim."
	}
	return snap
}
