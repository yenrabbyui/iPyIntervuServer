package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const bootstrapStartMessage = "start"

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
	if referer := os.Getenv("OPENROUTER_HTTP_REFERER"); referer != "" {
		req.Header.Set("HTTP-Referer", referer)
	}
	if title := os.Getenv("OPENROUTER_APP_TITLE"); title != "" {
		req.Header.Set("X-Title", title)
	}
}

func handleBootstrap(apiKey string, states *agentStateStore) http.HandlerFunc {
	client := &http.Client{}

	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

		var req bootstrapRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		state := states.getOrCreate(sessionID)
		prompt, _, _, err := buildSystemPrompt(state)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		payload, err := json.Marshal(chatCompletionRequest{
			Model: req.Model,
			Messages: []chatMessage{
				{Role: "system", Content: prompt},
				{Role: "user", Content: bootstrapStartMessage},
			},
		})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		upstream, err := http.NewRequestWithContext(
			r.Context(),
			http.MethodPost,
			openRouterURL,
			bytes.NewReader(payload),
		)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		openRouterHeaders(upstream, apiKey)

		resp, err := client.Do(upstream)
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		if resp.StatusCode != http.StatusOK {
			http.Error(w, string(body), resp.StatusCode)
			return
		}

		var completion openRouterCompletion
		if err := json.Unmarshal(body, &completion); err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		assistant := ""
		if len(completion.Choices) > 0 {
			assistant = completion.Choices[0].Message.Content
		}

		applyBootstrapState(state, assistant)
		states.set(sessionID, state)

		writeJSON(w, http.StatusOK, bootstrapResponse{Assistant: assistant})
	}
}

func handleChat(apiKey string, states *agentStateStore) http.HandlerFunc {
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

		state := states.getOrCreate(sessionID)
		userMessage := lastUserMessage(req.Messages)
		applyPreChatUserUpdate(state, userMessage)

		prompt, _, _, err := buildSystemPrompt(state)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		payload, err := prependSystemPrompt(body, prompt)
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, openRouterURL, bytes.NewReader(payload))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		openRouterHeaders(upstream, apiKey)

		resp, err := (&http.Client{}).Do(upstream)
		if err != nil {
			log.Printf("openrouter request failed: %v", err)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if req.Stream && strings.Contains(contentType, "text/event-stream") {
			copyHeader(w.Header(), resp.Header)
			w.WriteHeader(resp.StatusCode)
			assistant, streamErr := relaySSEAndCollect(w, resp.Body)
			if streamErr != nil {
				log.Printf("streaming response failed: %v", streamErr)
			}
			applyPostChatStateUpdate(state, userMessage, assistant)
			states.set(sessionID, state)
			return
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		assistant := extractAssistantContent(respBody)
		applyPostChatStateUpdate(state, userMessage, assistant)
		states.set(sessionID, state)

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
	}
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

func relaySSEAndCollect(dst io.Writer, src io.Reader) (string, error) {
	var assistant strings.Builder
	buf := make([]byte, 32*1024)
	remainder := ""

	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := remainder + string(buf[:n])
			parts := strings.Split(chunk, "\n\n")
			remainder = parts[len(parts)-1]
			for _, part := range parts[:len(parts)-1] {
				if _, writeErr := dst.Write([]byte(part + "\n\n")); writeErr != nil {
					return assistant.String(), writeErr
				}
				assistant.WriteString(parseSSEAssistantDelta(part))
			}
		}
		if err != nil {
			if remainder != "" {
				if _, writeErr := dst.Write([]byte(remainder)); writeErr != nil {
					return assistant.String(), writeErr
				}
				assistant.WriteString(parseSSEAssistantDelta(remainder))
			}
			if err == io.EOF {
				return assistant.String(), nil
			}
			return assistant.String(), err
		}
	}
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
