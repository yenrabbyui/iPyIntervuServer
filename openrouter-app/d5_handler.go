package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	d5BriefWait     = 2 * time.Second
	d5LabelJobsWait = 5 * time.Second
)

// handleD5Chat runs one /api/chat turn on the D5 engine (D5-interviewer-evaluator-design.md).
// The request and response contract is the original one, so the UI is unchanged.
// skipPreChat is set when a failed turn is resumed: its message was already applied.
func handleD5Chat(p chatRunParams, skipPreChat bool) {
	state := p.state
	sess := d5SessionFor(state, p.sessionID)
	sess.turnMu.Lock()
	defer sess.turnMu.Unlock()
	defer p.states.set(p.sessionID, state)

	msg := strings.TrimSpace(p.userMessage)
	phaseBefore := state.ConversationPhase

	// Setup is server-authored, exactly as in the original engine.
	if phaseBefore == phaseAwaitingMajor || phaseBefore == phaseAwaitingKeyConcept {
		if !skipPreChat {
			applyPreChatUserUpdate(state, msg)
		}
		if reply, ok := serverSetupReply(phaseBefore, state); ok {
			d5Respond(p, reply)
			return
		}
		// A week was chosen: the assessment starts with the first opening.
		sess.Personas = pickD5Personas(p.sessionID)
		d5RunOpening(p, sess, "")
		return
	}

	if !skipPreChat {
		state.MessageIndex++
		state.LastUserMessageRaw = msg
	}

	if state.ConversationPhase == phaseAssessmentResults {
		d5ResultsTurn(p, sess, msg, skipPreChat)
		return
	}
	if state.ConversationPhase != phaseAssessmentInProgress {
		d5Respond(p, setupReaskMajorMessage)
		return
	}

	if sess.NeedsOpening {
		d5RunOpening(p, sess, "")
		return
	}
	if isCoachingRequest(msg) {
		reply := d5CoachingAfterwards
		if q := d5PendingQuestion(sess); q != "" {
			reply += " To pick up where we left off: " + q
		}
		d5Respond(p, reply)
		return
	}

	brief := d5AwaitBrief(p, sess)
	clarification := d5AskedForClarification(msg)
	if !clarification && !skipPreChat {
		d5RecordAnswer(state, sess, msg)
	} else if clarification && !skipPreChat {
		sess.Transcript = append(sess.Transcript, d5Message{Role: "user", Content: msg, Mode: state.ActiveMode})
	}
	answerIndex, answerTarget := sess.AnswerIndex, sess.LastAskTarget

	move := d5ChooseMove(state, sess, brief, msg, clarification)
	log.Printf("[d5] move session=%s turn_id=%s mode=%s move=%s reason=%q answer_index=%d",
		truncateSessionID(p.sessionID), truncateTurnID(p.turnID), state.ActiveMode, move.Kind, move.Reason, answerIndex)

	if move.Kind == moveCloseMode {
		d5CloseMode(p, sess, msg, answerIndex, answerTarget)
		return
	}

	reply := d5Interview(p, sess, move, brief)
	d5Respond(p, reply)
	if move.Kind != moveClarify {
		d5LaunchEvaluator(p, sess, answerIndex, answerTarget)
	}
}

// d5AwaitBrief waits up to 2 s for an in-flight Evaluator brief, then returns the newest
// brief available (design §9).
func d5AwaitBrief(p chatRunParams, sess *d5Session) *d5Brief {
	sess.mu.Lock()
	ch := sess.briefInFlight
	sess.mu.Unlock()
	if ch != nil {
		started := time.Now()
		ready := true
		select {
		case <-ch:
		default:
			select {
			case <-ch:
			case <-time.After(d5BriefWait):
				ready = false
			}
		}
		log.Printf("[d5] brief_wait session=%s ready_before_next=%v waited_ms=%d",
			truncateSessionID(p.sessionID), ready, time.Since(started).Milliseconds())
	}
	return sess.latestBrief()
}

// d5Interview makes the live Interviewer call for a move, applies the post-checks, falls
// back to a fixed question on failure, and records the delivered reply.
func d5Interview(p chatRunParams, sess *d5Session, move d5Move, brief *d5Brief) string {
	state := p.state
	mode := state.ActiveMode
	system := d5InterviewerSystemPrompt(state, sess, mode, move, brief)
	req := interviewerRequest(p.req.Model, d5InterviewerMessages(system, sess, mode, false), move.MaxTokens)
	res, err := runInterviewerCall(p.apiKey, p.sessionID, req)
	fallback := d5FallbackQuestion(state, sess, brief, move)

	var reply string
	var issues []string
	if err != nil {
		log.Printf("[d5] interviewer_failed session=%s turn_id=%s move=%s attempts=%d err=%q",
			truncateSessionID(p.sessionID), truncateTurnID(p.turnID), move.Kind, res.Attempts, err.Error())
		reply = d5FallbackLeadIn + " " + fallback
		if move.Kind == moveClarify || move.Kind == moveRequestCode {
			reply = fallback
		}
	} else {
		checked := d5PostCheck(state, sess, mode, move, res.Content, fallback)
		reply, issues = checked.Reply, checked.Issues
	}
	d5LogInterviewer(p, move, res, len(issues))
	if len(issues) > 0 {
		log.Printf("[d5] postcheck session=%s turn_id=%s move=%s issues=%q",
			truncateSessionID(p.sessionID), truncateTurnID(p.turnID), move.Kind, issues)
	}
	sess.PendingIssues = issues
	d5ApplyInterviewerReply(state, sess, move, reply)
	return reply
}

func d5LogInterviewer(p chatRunParams, move d5Move, res d5CallResult, issues int) {
	log.Printf("[d5] interviewer_done session=%s turn_id=%s move=%s ms=%d ttft_ms=%d in_tokens=%d out_tokens=%d reasoning_tokens=%d cached_tokens=%d provider=%s attempts=%d cut=%v issues=%d",
		truncateSessionID(p.sessionID), truncateTurnID(p.turnID), move.Kind, res.Elapsed.Milliseconds(), res.TTFT.Milliseconds(),
		res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.CompletionTokensDetails.ReasoningTokens,
		res.Usage.PromptTokensDetails.CachedTokens, res.Provider, res.Attempts, res.Cut, issues)
}

// d5RunOpening opens the active mode with a live call (Phase 1; pre-drafted openings are
// Phase 2). prefix is sent before the introduction (the previous mode's closing line).
func d5RunOpening(p chatRunParams, sess *d5Session, prefix string) {
	state := p.state
	mode := state.ActiveMode
	move := d5OpeningMove(state, sess, mode)
	system := d5InterviewerSystemPrompt(state, sess, mode, move, nil)
	req := interviewerRequest(p.req.Model, d5InterviewerMessages(system, sess, mode, true), move.MaxTokens)
	res, err := runInterviewerCall(p.apiKey, p.sessionID, req)
	d5LogInterviewer(p, move, res, 0)

	content := strings.TrimSpace(res.Content)
	if err == nil && move.Kind == moveOpenFirst {
		name, domain, rest, ok := parseCompanyLine(content)
		if ok {
			sess.CompanyName, sess.CompanyDomain = name, domain
		} else {
			log.Printf("[d5] company_line_missing session=%s", truncateSessionID(p.sessionID))
		}
		content = rest
	}
	if err != nil || content == "" {
		log.Printf("[d5] opening_failed session=%s mode=%s err=%v", truncateSessionID(p.sessionID), mode, err)
		sess.NeedsOpening = true
		reply := "Sorry — I couldn't get the next part of the interview ready just now. Please send any message when you're ready to continue."
		if prefix != "" {
			reply = prefix + "\n\n" + reply
		}
		d5Respond(p, reply)
		return
	}

	sess.NeedsOpening = false
	sess.Material[mode] = content
	checked := d5PostCheck(state, sess, mode, move, content, "")
	if len(checked.Issues) > 0 {
		log.Printf("[d5] postcheck session=%s move=%s issues=%q", truncateSessionID(p.sessionID), move.Kind, checked.Issues)
	}
	sess.PendingIssues = checked.Issues
	opening := sess.personaIntro(mode) + "\n\n" + checked.Reply
	d5ApplyInterviewerReply(state, sess, move, opening)
	if prefix != "" {
		opening = prefix + "\n\n" + opening
	}
	d5Respond(p, opening)
}

// d5CloseMode closes the active mode: the final answer is labelled by the levels-only
// call, then either the next mode opens or Go grades and the results are sent.
func d5CloseMode(p chatRunParams, sess *d5Session, answer string, answerIndex int, answerTarget string) {
	state := p.state
	mode := state.ActiveMode
	question := d5QuestionBefore(sess, mode)
	vague := sess.VagueAnswers[answerIndex]
	final := d5FinalMode(state)

	if !vague && answerTarget != "" {
		msgs := d5LevelsMessages(state, mode, answerTarget, question, answer)
		req := levelsRequest(p.req.Model, msgs)
		job := func() {
			res, err := runBackgroundCall(p.apiKey, p.sessionID, req, d5LevelsTimeout, 1)
			labels := parseLevelsLine(res.Content, mode, answerIndex)
			log.Printf("[d5] levels_done session=%s mode=%s ms=%d labels=%d err=%v",
				truncateSessionID(p.sessionID), mode, res.Elapsed.Milliseconds(), len(labels), err)
			sess.addLabels(mode, labels)
		}
		if final {
			job()
		} else {
			sess.labelJobs.Add(1)
			go func() {
				defer sess.labelJobs.Done()
				job()
			}()
		}
	}

	if final {
		d5WaitLabelJobs(p, sess)
		applyD5Grades(state, sess.labelsSnapshot(), sess.CodePasted)
		d5FinishAssessment(state)
		log.Printf("[d5] results session=%s conceptual=%q code=%q bug=%q overall=%q",
			truncateSessionID(p.sessionID), state.ConceptualAssessmentBucket, state.CodeAssessmentBucket, state.BugAssessmentBucket, state.FinalRating)
		reply := d5ClosingInterview + "\n\n" + buildServerAssessmentResultsMessage(state)
		sess.Transcript = append(sess.Transcript, d5Message{Role: "assistant", Content: reply, Mode: phaseAssessmentResults})
		d5Respond(p, reply)
		return
	}

	d5AdvanceMode(state)
	d5RunOpening(p, sess, phaseClosingMessage)
}

func d5WaitLabelJobs(p chatRunParams, sess *d5Session) {
	done := make(chan struct{})
	go func() {
		sess.labelJobs.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d5LabelJobsWait):
		log.Printf("[d5] label_jobs_timeout session=%s", truncateSessionID(p.sessionID))
	}
}

// d5QuestionBefore returns the interviewer question the latest answer in mode replied to.
func d5QuestionBefore(sess *d5Session, mode string) string {
	msgs := sess.modeMessages(mode)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			if q := lastQuestionSentence(msgs[i].Content); q != "" {
				return q
			}
			return msgs[i].Content
		}
	}
	return ""
}

// d5LaunchEvaluator starts the background brief for the latest answer (design §5).
func d5LaunchEvaluator(p chatRunParams, sess *d5Session, answerIndex int, answerTarget string) {
	state := p.state
	mode := state.ActiveMode
	if answerIndex == 0 || state.ConversationPhase != phaseAssessmentInProgress {
		return
	}
	vague := sess.VagueAnswers[answerIndex]
	prev := sess.latestBrief()
	msgs := d5EvaluatorMessages(state, sess, mode, answerIndex, answerTarget, prev, sess.PendingIssues)
	req := evaluatorRequest(p.req.Model, msgs)
	done := make(chan struct{})
	sess.mu.Lock()
	sess.briefInFlight = done
	sess.mu.Unlock()

	go func() {
		defer close(done)
		res, err := runBackgroundCall(p.apiKey, p.sessionID, req, d5EvaluatorTimeout, 2)
		brief, ok := parseBrief(res.Content, mode, answerIndex)
		log.Printf("[d5] evaluator_done session=%s mode=%s answer_index=%d ms=%d out_tokens=%d reasoning_tokens=%d attempts=%d parsed=%v recommend=%s levels=%d err=%v",
			truncateSessionID(p.sessionID), mode, answerIndex, res.Elapsed.Milliseconds(), res.Usage.CompletionTokens,
			res.Usage.CompletionTokensDetails.ReasoningTokens, res.Attempts, ok, brief.Recommend, len(brief.Levels), err)
		if err != nil || !ok {
			return
		}
		if !vague {
			sess.addLabels(mode, brief.Levels)
		}
		sess.mu.Lock()
		defer sess.mu.Unlock()
		if sess.Brief == nil || answerIndex >= sess.Brief.AnswerIndex {
			sess.Brief = &brief
		}
		sess.ModeBriefs[mode] = &brief
		sess.Evidence[mode] = append(sess.Evidence[mode], brief.Evidence...)
	}()
}

// d5ResultsTurn handles messages after the results: coaching on request, otherwise a
// reminder of how to get it.
func d5ResultsTurn(p chatRunParams, sess *d5Session, msg string, skipPreChat bool) {
	state := p.state
	if state.ActiveMode != modeCoaching && !isCoachingRequest(msg) {
		d5Respond(p, d5ResultsReminder)
		return
	}
	firstCoaching := state.ActiveMode != modeCoaching
	state.ActiveMode = modeCoaching
	state.CoachingRequested = true
	if !skipPreChat {
		sess.Transcript = append(sess.Transcript, d5Message{Role: "user", Content: msg, Mode: modeCoaching})
	}

	msgs := []chatMessage{{Role: "system", Content: d5CoachingSystemPrompt(state, sess)}}
	history := sess.modeMessages(modeCoaching)
	if len(history) > 6 {
		history = history[len(history)-6:]
	}
	for _, m := range history {
		msgs = append(msgs, chatMessage{Role: m.Role, Content: m.Content})
	}
	req := interviewerRequest(p.req.Model, msgs, d5CoachingMaxTokens)
	res, err := runInterviewerCall(p.apiKey, p.sessionID, req)
	d5LogInterviewer(p, d5Move{Kind: moveCoaching}, res, 0)

	reply := strings.TrimSpace(res.Content)
	if err != nil || reply == "" {
		reply = "Sorry — I couldn't put my feedback together just now. Please ask again in a moment."
	} else if firstCoaching {
		reply = sess.personaIntro(modeCoaching) + "\n\n" + reply
	}
	sess.Transcript = append(sess.Transcript, d5Message{Role: "assistant", Content: reply, Mode: modeCoaching})
	d5Respond(p, reply)
}

// d5Respond writes the reply in the OpenRouter completion shape the UI already reads and
// completes the turn so replays and followers get the same reply.
func d5Respond(p chatRunParams, content string) {
	body, err := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content}}},
	})
	if err != nil {
		http.Error(p.w, "internal error", http.StatusInternalServerError)
		if p.managesTurn() {
			p.turns.failTurn(p.sessionID, p.turnID, content, content, nil, http.StatusInternalServerError)
		}
		return
	}
	p.state.LastAssistantSummary = truncateSummary(content, 500)
	writeChatResponse(p.w, body, http.StatusOK)
	if p.managesTurn() {
		p.turns.completeTurn(p.sessionID, p.turnID, content, content, body, http.StatusOK)
	}
}
