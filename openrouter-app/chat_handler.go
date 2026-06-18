package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func turnRoleName(role turnRole) string {
	switch role {
	case turnRoleLeader:
		return "leader"
	case turnRoleFollower:
		return "follower"
	case turnRoleReplayCompleted:
		return "replay_completed"
	case turnRoleReplayFailed:
		return "replay_failed"
	case turnRoleResumeLeader:
		return "resume_leader"
	default:
		return "unknown"
	}
}

func truncateTurnID(turnID string) string {
	if len(turnID) <= 12 {
		return turnID
	}
	return turnID[:12]
}

func handleChat(apiKey string, states *agentStateStore, turns *turnStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		turnID := strings.TrimSpace(r.Header.Get(turnIDHeader))
		state := states.getOrCreate(sessionID)
		userMessage := lastUserMessage(req.Messages)

		var turnRec *chatTurnRecord
		var turnRole turnRole
		skipPreChat := false

		if turnID != "" {
			acquired := turns.acquire(sessionID, turnID)
			turnRec = acquired.record
			turnRole = acquired.role

			log.Printf("[openrouter] turn_acquire session=%s turn_id=%s role=%s status=%s",
				truncateSessionID(sessionID), truncateTurnID(turnID), turnRoleName(turnRole), turnRec.status)

			switch turnRole {
			case turnRoleFollower:
				writeSSEStreamHeaders(w)
				if err := turnRec.stream.follow(w, r.Context()); err != nil && err != context.Canceled {
					log.Printf("[openrouter] turn_follow_failed session=%s turn_id=%s err=%q",
						truncateSessionID(sessionID), truncateTurnID(turnID), err.Error())
				}
				return
			case turnRoleReplayCompleted:
				writeSSEStreamHeaders(w)
				visible := turnRec.stream.visibleAssistant
				if visible == "" {
					visible = clientVisibleAssistantContent(turnRec.stream.rawAssistant)
				}
				_ = writeSSEReplayFromVisible(w, visible)
				return
			case turnRoleReplayFailed:
				writeSSEStreamHeaders(w)
				if visible := turnRec.stream.visibleAssistant; visible != "" {
					_ = writeSSEReplayFromVisible(w, visible)
				}
				return
			case turnRoleResumeLeader:
				skipPreChat = true
				writeSSEStreamHeaders(w)
				if err := turnRec.stream.replayExistingTo(w); err != nil {
					log.Printf("[openrouter] turn_resume_replay_failed session=%s turn_id=%s err=%q",
						truncateSessionID(sessionID), truncateTurnID(turnID), err.Error())
					return
				}
			case turnRoleLeader:
				// apply pre-chat below
			}
		}

		if !skipPreChat {
			applyPreChatUserUpdate(state, userMessage)
		}
		states.set(sessionID, state)

		log.Printf("[openrouter] chat_start session=%s turn_id=%s active_mode=%s phase=%s message_index=%d stream=%v",
			truncateSessionID(sessionID), truncateTurnID(turnID), state.ActiveMode, state.ConversationPhase, state.MessageIndex, req.Stream)

		runChatRequest(chatRunParams{
			sessionID: sessionID,
			turnID:    turnID,
			turnRec:   turnRec,
			turnRole:  turnRole,
			turns:     turns,
			states:    states,
			state:     state,
			req:       req,
			userMessage: userMessage,
			apiKey:    apiKey,
			r:         r,
			w:         w,
		})
	}
}

type chatRunParams struct {
	sessionID   string
	turnID      string
	turnRec     *chatTurnRecord
	turnRole    turnRole
	turns       *turnStore
	states      *agentStateStore
	state       *AgentSessionState
	req         chatCompletionRequest
	userMessage string
	apiKey      string
	r           *http.Request
	w           http.ResponseWriter
}

func (p *chatRunParams) sharedStream() *sharedTurnStream {
	if p.turnRec == nil {
		return nil
	}
	return p.turnRec.stream
}

func (p *chatRunParams) managesTurn() bool {
	if p.turnID == "" || p.turnRec == nil {
		return false
	}
	return p.turnRole == turnRoleLeader || p.turnRole == turnRoleResumeLeader
}

// upstreamContext keeps OpenRouter work alive when the browser drops the SSE
// connection but the same turn may reconnect as a follower.
func (p *chatRunParams) upstreamContext() context.Context {
	if p.managesTurn() && p.turnID != "" {
		return context.WithoutCancel(p.r.Context())
	}
	return p.r.Context()
}

func (p *chatRunParams) finalizeTurn(rawAssistant string, failed bool) {
	if !p.managesTurn() {
		return
	}
	visible := clientVisibleAssistantContent(rawAssistant)
	if failed {
		p.turns.failTurn(p.sessionID, p.turnID, rawAssistant, visible)
		log.Printf("[openrouter] turn_failed session=%s turn_id=%s visible_chars=%d",
			truncateSessionID(p.sessionID), truncateTurnID(p.turnID), len(visible))
		return
	}
	p.turns.completeTurn(p.sessionID, p.turnID, rawAssistant, visible)
	log.Printf("[openrouter] turn_completed session=%s turn_id=%s visible_chars=%d",
		truncateSessionID(p.sessionID), truncateTurnID(p.turnID), len(visible))
}

func (p *chatRunParams) tryScheduleFollowUpTurn(
	followUp assistantTurnFollowUp,
	assistant string,
	turnMessages *[]chatMessage,
	turnUserMessage *string,
	modeContinuations *int,
	bucketSyncAttempted *bool,
	streaming bool,
) bool {
	if !followUp.ContinueTurn || followUp.Handoff == "" {
		return false
	}
	if followUp.Kind == "mode" || followUp.Kind == "results" {
		*modeContinuations++
	}
	if followUp.Kind == "assessment_sync" {
		*bucketSyncAttempted = true
		log.Printf("[openrouter] assessment_sync_retry session=%s turn_id=%s active_mode=%s",
			truncateSessionID(p.sessionID), truncateTurnID(p.turnID), p.state.ActiveMode)
	}
	if streaming {
		writeSSEKeepalive(p.w)
	}
	*turnMessages = append(*turnMessages,
		chatMessage{Role: "assistant", Content: assistant},
		chatMessage{Role: "user", Content: followUp.Handoff},
	)
	*turnUserMessage = followUp.Handoff
	return true
}

func runChatRequest(p chatRunParams) {
	chatStarted := time.Now()
	modeContinuations := 0
	bucketSyncAttempted := false

	turnMessages := append([]chatMessage(nil), p.req.Messages...)
	turnUserMessage := p.userMessage
	streaming := p.req.Stream
	var combinedAssistant strings.Builder
	streamStarted := false
	if p.turnRole == turnRoleResumeLeader && p.sharedStream() != nil {
		combinedAssistant.WriteString(p.sharedStream().rawAssistant)
	}

	defer func() {
		if !p.managesTurn() || streamStarted {
			return
		}
		raw := combinedAssistant.String()
		if raw == "" && p.sharedStream() != nil {
			raw = p.sharedStream().rawAssistant
		}
		if raw != "" {
			p.finalizeTurn(raw, true)
		}
	}()

	for turn := 0; turn < maxChatInternalTurns; turn++ {
		prompt, _, _, err := buildSystemPrompt(p.state)
		if err != nil {
			http.Error(p.w, "internal error", http.StatusInternalServerError)
			return
		}

		payload, err := json.Marshal(chatCompletionRequest{
			Model:    resolveChatModel(p.req.Model),
			Messages: prependSystemMessage(turnMessages, prompt),
			Stream:   streaming,
		})
		if err != nil {
			http.Error(p.w, "internal error", http.StatusInternalServerError)
			return
		}

		if streaming {
			streamHeadersSent := turn > 0 || p.turnRole == turnRoleResumeLeader
			var assistant string
			logCtx := newOpenRouterLogCtx(p.sessionID, p.turnID, "chat_stream", turn)

		streamAttempt:
			for attempt := 0; attempt < openRouterMaxAttempts; attempt++ {
				if err := waitOpenRouterRetry(p.upstreamContext(), attempt, logCtx); err != nil {
					logOpenRouterFailure(logCtx, attempt, "context", err, streamHeadersSent, "stream_wait_retry")
					if p.managesTurn() {
						p.finalizeTurn(combinedAssistant.String(), true)
					}
					return
				}

				attemptStarted := time.Now()
				logOpenRouterStart(logCtx, attempt, true)

				resp, err := postOpenRouterOnce(p.upstreamContext(), p.apiKey, payload)
				if err != nil {
					logOpenRouterFailure(logCtx, attempt, "connect", err, streamHeadersSent, "post_openrouter_once")
					if attempt < openRouterMaxAttempts-1 && isRetryableOpenRouterError(err) {
						logOpenRouterRetry(logCtx, attempt, classifyOpenRouterError(err, false))
						continue
					}
					if !streamHeadersSent {
						http.Error(p.w, "upstream error", http.StatusBadGateway)
					}
					if p.managesTurn() {
						p.finalizeTurn(combinedAssistant.String(), true)
					}
					return
				}

				contentType := resp.Header.Get("Content-Type")
				connectMs := time.Since(attemptStarted).Milliseconds()
				logOpenRouterConnected(logCtx, attempt, resp.StatusCode, contentType, connectMs)

				if !strings.Contains(contentType, "text/event-stream") {
					respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
					resp.Body.Close()
					if readErr != nil {
						logOpenRouterFailure(logCtx, attempt, "upstream_read", readErr, streamHeadersSent, "non_sse_body")
						if attempt < openRouterMaxAttempts-1 && isRetryableOpenRouterError(readErr) {
							logOpenRouterRetry(logCtx, attempt, classifyOpenRouterError(readErr, false))
							continue
						}
						http.Error(p.w, "upstream error", http.StatusBadGateway)
						if p.managesTurn() {
							p.finalizeTurn(combinedAssistant.String(), true)
						}
						return
					}
					if isRetryableOpenRouterStatus(resp.StatusCode) && attempt < openRouterMaxAttempts-1 {
						logOpenRouterStatusRetry(logCtx, attempt, resp.StatusCode, string(respBody))
						continue
					}
					assistant = extractAssistantContent(respBody)
					goto streamComplete
				}

				if !streamHeadersSent {
					copyHeader(p.w.Header(), resp.Header)
					p.w.WriteHeader(resp.StatusCode)
					streamHeadersSent = true
					streamStarted = true
				}

				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
					resp.Body.Close()
					if isRetryableOpenRouterStatus(resp.StatusCode) && attempt < openRouterMaxAttempts-1 {
						logOpenRouterStatusRetry(logCtx, attempt, resp.StatusCode, string(body))
						continue
					}
					log.Printf("[openrouter] stream_bad_status session=%s turn_id=%s mode_turn=%d status=%d body_preview=%q",
						truncateSessionID(p.sessionID), truncateTurnID(p.turnID), turn, resp.StatusCode, truncateForLog(string(body), 200))
					if turn == 0 {
						_, _ = p.w.Write(body)
					}
					if p.managesTurn() {
						p.finalizeTurn(combinedAssistant.String(), true)
					}
					return
				}

				relayStarted := time.Now()
				result := relaySSEFilteredWithBuffer(p.w, resp.Body, p.sharedStream())
				resp.Body.Close()
				logOpenRouterStreamDone(logCtx, attempt, result, time.Since(relayStarted).Milliseconds())

				if result.err != nil {
					if isRetryableStreamRelayError(result.err, result.sentToClient) && attempt < openRouterMaxAttempts-1 {
						logOpenRouterRetry(logCtx, attempt, result.failurePoint+":"+classifyOpenRouterError(result.err, result.sentToClient))
						continue streamAttempt
					}
					if result.assistant != "" {
						assistant = result.assistant
						break streamAttempt
					}
					if p.managesTurn() {
						p.finalizeTurn(combinedAssistant.String(), true)
					}
					return
				}
				assistant = result.assistant
				break streamAttempt
			}

			if assistant == "" && !streamHeadersSent {
				log.Printf("[openrouter] chat_stream_exhausted session=%s turn_id=%s mode_turn=%d attempts=%d",
					truncateSessionID(p.sessionID), truncateTurnID(p.turnID), turn, openRouterMaxAttempts)
				http.Error(p.w, "upstream error", http.StatusBadGateway)
				if p.managesTurn() {
					p.finalizeTurn(combinedAssistant.String(), true)
				}
				return
			}

		streamComplete:
			if combinedAssistant.Len() > 0 {
				combinedAssistant.WriteString("\n\n")
			}
			combinedAssistant.WriteString(assistant)

			syncLog := &ipySyncLogContext{
				sessionID:  p.sessionID,
				turnID:     p.turnID,
				modeTurn:   turn,
				activeMode: p.state.ActiveMode,
				phase:      p.state.ConversationPhase,
			}
			followUp := postProcessAssistantTurn(p.state, assistant, bucketSyncAttempted, syncLog)
			p.states.set(p.sessionID, p.state)
			if p.tryScheduleFollowUpTurn(followUp, assistant, &turnMessages, &turnUserMessage, &modeContinuations, &bucketSyncAttempted, streaming) {
				continue
			}
			if p.managesTurn() {
				p.finalizeTurn(combinedAssistant.String(), false)
			}
			logChatRequestDone(p.sessionID, p.turnID, time.Since(chatStarted).Milliseconds(), modeContinuations)
			return
		}

		logCtx := newOpenRouterLogCtx(p.sessionID, p.turnID, "chat_body", turn)
		respBody, statusCode, err := readOpenRouterBodyWithRetry(p.upstreamContext(), p.apiKey, payload, logCtx)
		if err != nil {
			log.Printf("[openrouter] chat_body_failed session=%s turn_id=%s mode_turn=%d err=%q",
				truncateSessionID(p.sessionID), truncateTurnID(p.turnID), turn, err.Error())
			http.Error(p.w, "upstream error", http.StatusBadGateway)
			if p.managesTurn() {
				p.finalizeTurn(combinedAssistant.String(), true)
			}
			return
		}

		if statusCode != http.StatusOK {
			http.Error(p.w, string(respBody), statusCode)
			if p.managesTurn() {
				p.finalizeTurn(combinedAssistant.String(), true)
			}
			return
		}

		assistant := extractAssistantContent(respBody)
		if combinedAssistant.Len() > 0 {
			combinedAssistant.WriteString("\n\n")
		}
		combinedAssistant.WriteString(assistant)

		syncLog := &ipySyncLogContext{
			sessionID:  p.sessionID,
			turnID:     p.turnID,
			modeTurn:   turn,
			activeMode: p.state.ActiveMode,
			phase:      p.state.ConversationPhase,
		}
		followUp := postProcessAssistantTurn(p.state, assistant, bucketSyncAttempted, syncLog)
		p.states.set(p.sessionID, p.state)

		if p.tryScheduleFollowUpTurn(followUp, assistant, &turnMessages, &turnUserMessage, &modeContinuations, &bucketSyncAttempted, streaming) {
			continue
		}

		strippedBody, err := replaceAssistantContent(respBody, stripIPyIntervuTail(combinedAssistant.String()))
		if err != nil {
			strippedBody = respBody
		}
		p.w.Header().Set("Content-Type", "application/json")
		p.w.WriteHeader(statusCode)
		_, _ = p.w.Write(strippedBody)
		if p.managesTurn() {
			p.finalizeTurn(combinedAssistant.String(), false)
		}
		logChatRequestDone(p.sessionID, p.turnID, time.Since(chatStarted).Milliseconds(), modeContinuations)
		return
	}

	if p.managesTurn() {
		p.finalizeTurn(combinedAssistant.String(), false)
	}
	logChatRequestDone(p.sessionID, p.turnID, time.Since(chatStarted).Milliseconds(), modeContinuations)
}
