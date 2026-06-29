package main

import (
	"log"
	"strings"
)

type assessmentViolations struct {
	Content               bool
	SevereContent         bool
	MissingSync           bool
	CompleteWithoutBucket bool
}

func (v assessmentViolations) Any() bool {
	return v.Content || v.SevereContent || v.MissingSync || v.CompleteWithoutBucket
}

func (v assessmentViolations) NeedsTruncateOnly() bool {
	return v.Content && !v.SevereContent && !v.MissingSync && !v.CompleteWithoutBucket
}

func (v assessmentViolations) NeedsCorrectiveRetry() bool {
	return v.MissingSync || v.CompleteWithoutBucket || v.SevereContent
}

func (v assessmentViolations) StillInvalidAfterRetry() bool {
	return v.Any()
}

func assessmentViolationCheckEnabled(state *AgentSessionState) bool {
	return questionGuardEnabledForState(state)
}

func detectAssessmentViolations(state *AgentSessionState, assistant string) assessmentViolations {
	var v assessmentViolations
	if !assessmentViolationCheckEnabled(state) {
		return v
	}

	trimmed := strings.TrimSpace(stripIPyIntervuTail(assistant))
	if !hasIPyIntervuBlock(assistant) {
		v.MissingSync = true
	}
	if shouldForceAssessmentSync(state, assistant) {
		v.CompleteWithoutBucket = true
	}
	if looksLikeSelfAnsweredQuestion(trimmed) || looksLikeCompositeAssessmentReply(trimmed) {
		v.Content = true
		if looksLikeSevereCompositeReply(trimmed) {
			v.SevereContent = true
		}
	}
	return v
}

func logAssessmentViolations(sessionID, turnID string, modeTurn int, v assessmentViolations) {
	if !v.Any() {
		return
	}
	log.Printf("[openrouter] assessment_violations session=%s turn_id=%s mode_turn=%d content=%v severe_content=%v missing_sync=%v complete_without_bucket=%v",
		truncateSessionID(sessionID), truncateTurnID(turnID), modeTurn, v.Content, v.SevereContent, v.MissingSync, v.CompleteWithoutBucket)
}

func buildViolationIssueText(v assessmentViolations, phaseField, bucketField string) string {
	var issues []string
	if v.Content {
		issues = append(issues, "answered your own question, simulated the student's reply, stacked multiple interview questions, or continued as if the student already responded")
	}
	if v.MissingSync {
		issues = append(issues, "omitted the required ```_ipyintervu``` sync block")
	}
	if v.CompleteWithoutBucket {
		issues = append(issues, "did not send \""+phaseField+"\": \"complete\" with "+bucketField)
	}
	return strings.Join(issues, "; ")
}

func assessmentFinishRule(phaseField, bucketField string) string {
	return "While still interviewing: {\"" + phaseField + "\": \"in_progress\"} with no bucket. When finishing this mode: {\"" + phaseField + "\": \"complete\", \"" + bucketField + "\": \"Not Ready Yet\"|\"Competent\"|\"Exceptional\"}."
}

func buildRewindCorrectiveHandoff(state *AgentSessionState, label, issueText, finishRule string) string {
	_ = state
	return "[System: Your last " + label + " reply violated assessment protocol because it " + issueText + ". Re-present the SAME concrete scenario (or code task / bug snippet) from your previous reply — do not drop or shorten it. Then one brief neutral lead-in (optional) plus exactly ONE student-directed interview question, OR (if this mode is finished) one brief neutral closing sentence with no new question. Do NOT write what the student would say, do not put an answer on the line after your question, and do not stack acknowledgments with extra questions. Do NOT mention re-presenting, correcting, server retries, or [System] in user-facing text — continue in persona voice only. End with ```_ipyintervu``` as the last lines. " + finishRule + " Use exact bucket strings. Then stop and wait for the student unless you sent complete plus bucket.]"
}

func buildForwardMissingSyncCorrectiveHandoff(state *AgentSessionState, label, finishRule string) string {
	forward := forwardInterviewMoveGuidance(state)
	return "[System: Your last " + label + " reply omitted the required ```_ipyintervu``` sync block. The student already answered your previous question — do NOT re-present the opening scenario or repeat the same question. " + forward + " Optional brief neutral lead-in (Got it./Thanks.) plus exactly ONE new interview question, OR (if this mode is finished) one brief neutral closing sentence with no new question. Do NOT mention re-presenting, correcting, server retries, or [System] in user-facing text — continue in persona voice only. End with ```_ipyintervu``` as the last lines. " + finishRule + " Use exact bucket strings. Then stop and wait for the student unless you sent complete plus bucket.]"
}

func buildCompleteWithoutBucketHandoff(state *AgentSessionState, label, issueText, finishRule string) string {
	_ = state
	return "[System: Your last " + label + " reply violated assessment protocol because it " + issueText + ". If you are finishing this mode, send one brief neutral closing sentence with no new question and end with ```_ipyintervu``` containing \"complete\" plus the bucket in the same reply. If you are still interviewing, use \"in_progress\" only. Do NOT mention correcting, server retries, or [System] in user-facing text. " + finishRule + " Use exact bucket strings. Then stop.]"
}

func buildUnifiedCorrectiveHandoff(state *AgentSessionState, v assessmentViolations) string {
	phaseField, bucketField, label := currentModeSyncFields(state)
	if phaseField == "" {
		return ""
	}

	issueText := buildViolationIssueText(v, phaseField, bucketField)
	finishRule := assessmentFinishRule(phaseField, bucketField)

	if v.CompleteWithoutBucket {
		return buildCompleteWithoutBucketHandoff(state, label, issueText, finishRule)
	}
	if v.Content {
		return buildRewindCorrectiveHandoff(state, label, issueText, finishRule)
	}
	if v.MissingSync && isFollowUpAssessmentTurn(state) {
		return buildForwardMissingSyncCorrectiveHandoff(state, label, finishRule)
	}
	if v.MissingSync {
		return buildRewindCorrectiveHandoff(state, label, issueText, finishRule)
	}
	return ""
}

func buildServerAssessmentResultsMessage(state *AgentSessionState) string {
	conceptual := state.ConceptualAssessmentBucket
	code := state.CodeAssessmentBucket
	bug := state.BugAssessmentBucket
	if state.isProblemDecompositionWeek() {
		code = bucketNA
		bug = bucketNA
	}
	if code == "" {
		code = bucketNA
	}
	if bug == "" {
		bug = bucketNA
	}
	selected := state.SelectedKeyConcept
	if selected == "" {
		selected = "your selected key concept"
	}
	return strings.Join([]string{
		"Assessment Results — " + selected,
		"",
		"- Conceptual Assessment: " + conceptual,
		"- Code Assessment: " + code,
		"- Bug Assessment: " + bug,
		"",
		"Overall Rating: " + state.FinalRating,
	}, "\n")
}

func buildAssessmentTurnFailureMessage() string {
	return "I'm sorry — I hit a temporary issue finishing that assessment step. Please send your last answer again or ask to continue, and we'll pick up from here."
}
