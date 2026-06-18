package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

const bootstrapStartMessage = "start"
const defaultChatModel = "deepseek/deepseek-v4-flash"

func resolveChatModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return defaultChatModel
	}
	return model
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type bootstrapRequest struct {
	Model string `json:"model"`
}

type bootstrapResponse struct {
	Assistant string `json:"assistant"`
}

type openRouterCompletion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openRouterStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func lastUserMessage(messages []chatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func prependSystemPrompt(body []byte, prompt string) ([]byte, error) {
	var req chatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	system := chatMessage{Role: "system", Content: prompt}
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		req.Messages[0] = system
	} else {
		req.Messages = append([]chatMessage{system}, req.Messages...)
	}

	return json.Marshal(req)
}

func openRouterHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if referer := os.Getenv("OPENROUTER_HTTP_REFERER"); referer != "https://aalang.org" {
		req.Header.Set("HTTP-Referer", referer)
	}
	if title := os.Getenv("OPENROUTER_APP_TITLE"); title != "iPyInterVu" {
		req.Header.Set("X-Title", title)
	}
}

func handleBootstrap(apiKey string, states *agentStateStore, turns *turnStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

		var req bootstrapRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		model := resolveChatModel(req.Model)

		turnID := strings.TrimSpace(r.Header.Get(turnIDHeader))
		if turnID == bootstrapTurnID {
			if assistant, ok := turns.getBootstrapAssistant(sessionID); ok {
				writeJSON(w, http.StatusOK, bootstrapResponse{Assistant: assistant})
				return
			}
			if !turns.beginBootstrap(sessionID) {
				assistant, err := turns.waitBootstrap(r.Context(), sessionID)
				if err != nil {
					http.Error(w, "upstream error", http.StatusBadGateway)
					return
				}
				writeJSON(w, http.StatusOK, bootstrapResponse{Assistant: assistant})
				return
			}
		}

		state := states.getOrCreate(sessionID)
		prompt, _, _, err := buildSystemPrompt(state)
		if err != nil {
			if turnID == bootstrapTurnID {
				turns.cancelBootstrap(sessionID)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		payload, err := json.Marshal(chatCompletionRequest{
			Model: model,
			Messages: []chatMessage{
				{Role: "system", Content: prompt},
				{Role: "user", Content: bootstrapStartMessage},
			},
		})
		if err != nil {
			if turnID == bootstrapTurnID {
				turns.cancelBootstrap(sessionID)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		body, statusCode, err := readOpenRouterBodyWithRetry(r.Context(), apiKey, payload, newOpenRouterLogCtx(sessionID, turnID, "bootstrap", 0))
		if err != nil {
			log.Printf("[openrouter] bootstrap_failed session=%s turn_id=%s err=%q", truncateSessionID(sessionID), truncateTurnID(turnID), err.Error())
			if turnID == bootstrapTurnID {
				turns.cancelBootstrap(sessionID)
			}
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		if statusCode != http.StatusOK {
			if turnID == bootstrapTurnID {
				turns.cancelBootstrap(sessionID)
			}
			http.Error(w, string(body), statusCode)
			return
		}

		var completion openRouterCompletion
		if err := json.Unmarshal(body, &completion); err != nil {
			if turnID == bootstrapTurnID {
				turns.cancelBootstrap(sessionID)
			}
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		assistant := ""
		if len(completion.Choices) > 0 {
			assistant = completion.Choices[0].Message.Content
		}

		assistant = stripIPyIntervuTail(assistant)
		applyBootstrapState(state, assistant)
		states.set(sessionID, state)

		if turnID == bootstrapTurnID {
			turns.setBootstrapAssistant(sessionID, assistant)
		}

		writeJSON(w, http.StatusOK, bootstrapResponse{Assistant: assistant})
	}
}

func prependSystemMessage(messages []chatMessage, prompt string) []chatMessage {
	system := chatMessage{Role: "system", Content: prompt}
	if len(messages) > 0 && messages[0].Role == "system" {
		out := make([]chatMessage, len(messages))
		copy(out, messages)
		out[0] = system
		return out
	}
	return append([]chatMessage{system}, messages...)
}

func extractAssistantContent(body []byte) string {
	var completion openRouterCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		return ""
	}
	if len(completion.Choices) == 0 {
		return ""
	}
	return completion.Choices[0].Message.Content
}

func parseSSEAssistantDelta(part string) string {
	var collected strings.Builder
	for _, line := range strings.Split(part, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk openRouterStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			collected.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	return collected.String()
}

func handleSessionState(states *agentStateStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		state, ok := states.get(sessionID)
		if !ok {
			state = newAgentSessionState()
		}
		writeJSON(w, http.StatusOK, state)
	}
}
