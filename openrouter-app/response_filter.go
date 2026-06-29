package main

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var ipyintervuFenceStripPattern = regexp.MustCompile("(?is)```(?:json)?\\s*_ipy(?:intervu)?\\s*\\n.*?\\n```")

func stripTrailingPartialIPyFence(content string) string {
	content = stripStraySyncFenceTail(content)
	if strings.Count(content, "```") == 0 {
		return content
	}
	if strings.Count(content, "```")%2 == 0 {
		return content
	}
	open := strings.LastIndex(content, "```")
	after := strings.TrimSpace(content[open+3:])
	if after == "" ||
		strings.HasPrefix(after, "json") ||
		strings.HasPrefix(after, "_") ||
		strings.HasPrefix(strings.ToLower(after), "_ipy") {
		return strings.TrimRight(content[:open], " \t\n\r")
	}
	return content
}

func stripStraySyncFenceTail(content string) string {
	content = strings.TrimRight(content, " \t\n\r")
	for {
		i := len(content) - 1
		for i >= 0 && content[i] == '`' {
			i--
		}
		trailing := content[i+1:]
		if trailing == "" || len(trailing) >= 3 {
			break
		}
		content = strings.TrimRight(content[:i+1], " \t\n\r")
	}
	return content
}

// stripIPyIntervuTail removes complete _ipyintervu sync blocks from assistant text.
func stripIPyIntervuTail(content string) string {
	s := ipyintervuFenceStripPattern.ReplaceAllString(content, "")
	return strings.TrimRight(s, " \t\n\r")
}

// clientVisibleAssistantContent is safe to stream/display (complete blocks removed, partial fence held back).
func clientVisibleAssistantContent(accumulated string) string {
	return stripTrailingPartialIPyFence(stripIPyIntervuTail(accumulated))
}

// streamingVisibleDelta returns newly visible text for SSE relay and updates sentVisible.
// When question-guard truncation shortens visible text, sentVisible is clamped so slice
// bounds cannot panic mid-stream (which would leave the turn stuck in_progress).
func streamingVisibleDelta(visible string, sentVisible *int) string {
	if *sentVisible > len(visible) {
		*sentVisible = len(visible)
	}
	delta := visible[*sentVisible:]
	*sentVisible = len(visible)
	return delta
}

func replaceAssistantContent(body []byte, content string) ([]byte, error) {
	var completion openRouterCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		return body, err
	}
	if len(completion.Choices) == 0 {
		return body, nil
	}
	completion.Choices[0].Message.Content = content
	return json.Marshal(completion)
}

func rewriteSSEContentDelta(part string, newContent string) string {
	lines := strings.Split(part, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk openRouterStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if chunk.Choices[0].Delta.Content == "" && newContent == "" {
			continue
		}
		chunk.Choices[0].Delta.Content = newContent
		encoded, err := json.Marshal(chunk)
		if err != nil {
			continue
		}
		lines[i] = "data: " + string(encoded)
		changed = true
	}
	if !changed {
		return part
	}
	return strings.Join(lines, "\n")
}

func writeSSEKeepalive(w http.ResponseWriter) {
	_, _ = w.Write([]byte(": keepalive\n\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

type relaySSEResult struct {
	assistant               string
	sentToClient            bool
	bufferedPreResultsStream bool
	failurePoint            string // upstream_read, client_write, none
	err                     error
}

func relaySSEFiltered(dst io.Writer, src io.Reader) relaySSEResult {
	return relaySSEFilteredWithBuffer(dst, src, nil, nil)
}

func relaySSEFilteredWithBuffer(dst io.Writer, src io.Reader, shared *sharedTurnStream, state *AgentSessionState) relaySSEResult {
	stopKeepalive := startSSERelayKeepalive(dst)
	defer stopKeepalive()

	var accumulated strings.Builder
	var sentVisible int
	var sentToClient bool
	bufferPreResults := shouldBufferPreResultsStream(state)
	clientGone := false
	buf := make([]byte, 32*1024)
	remainder := ""

	flushPart := func(part string) error {
		delta := parseSSEAssistantDelta(part)
		outPart := part
		if delta != "" {
			accumulated.WriteString(delta)
			visible := clientVisibleAssistantContentGuarded(accumulated.String(), state)
			newDelta := streamingVisibleDelta(visible, &sentVisible)
			if newDelta == "" {
				return nil
			}
			outPart = rewriteSSEContentDelta(part, newDelta)
		}
		if shared != nil {
			shared.appendChunk(outPart, accumulated.String(), clientVisibleAssistantContentGuarded(accumulated.String(), state))
		}
		if clientGone || bufferPreResults {
			return nil
		}
		_, err := dst.Write([]byte(outPart + "\n\n"))
		if err != nil {
			clientGone = true
			if shared != nil {
				return nil
			}
			return err
		}
		sentToClient = true
		if flusher, ok := dst.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}

	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := remainder + string(buf[:n])
			parts := strings.Split(chunk, "\n\n")
			remainder = parts[len(parts)-1]
			for _, part := range parts[:len(parts)-1] {
				if flushErr := flushPart(part); flushErr != nil {
					return relaySSEResult{
						assistant:    accumulated.String(),
						sentToClient: sentToClient,
						failurePoint: "client_write",
						err:          flushErr,
					}
				}
			}
		}
		if err != nil {
			if remainder != "" {
				if flushErr := flushPart(remainder); flushErr != nil {
					return relaySSEResult{
						assistant:    accumulated.String(),
						sentToClient: sentToClient,
						failurePoint: "client_write",
						err:          flushErr,
					}
				}
			}
			if err == io.EOF {
				return relaySSEResult{
					assistant:               accumulated.String(),
					sentToClient:            sentToClient,
					bufferedPreResultsStream: bufferPreResults,
					failurePoint:            "none",
				}
			}
			return relaySSEResult{
				assistant:    accumulated.String(),
				sentToClient: sentToClient,
				failurePoint: "upstream_read",
				err:          err,
			}
		}
	}
}

func startSSERelayKeepalive(dst io.Writer) func() {
	w, ok := dst.(http.ResponseWriter)
	if !ok {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(12 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeSSEKeepalive(w)
			}
		}
	}()
	return func() {
		close(done)
	}
}
