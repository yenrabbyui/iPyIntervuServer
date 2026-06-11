package main

import (
	"encoding/json"
	"net/http"
)

const sseReplayChunkSize = 64

func writeSSEStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

func writeSSEReplayFromVisible(w http.ResponseWriter, visible string) error {
	for i := 0; i < len(visible); i += sseReplayChunkSize {
		end := i + sseReplayChunkSize
		if end > len(visible) {
			end = len(visible)
		}
		if err := writeSSEContentDelta(w, visible[i:end]); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("data: [DONE]\n\n"))
	flushResponseWriter(w)
	return err
}

func writeSSEContentDelta(w http.ResponseWriter, content string) error {
	payload, err := json.Marshal(openRouterStreamChunk{
		Choices: []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		}{{Delta: struct {
			Content string `json:"content"`
		}{Content: content}}},
	})
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return err
	}
	flushResponseWriter(w)
	return nil
}
