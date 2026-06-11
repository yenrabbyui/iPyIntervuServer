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

// partialIPyIntervuFence matches a trailing incomplete _ipy / _ipyintervu fenced block.
var partialIPyIntervuFence = regexp.MustCompile("(?is)```(?:json\\s*)?(?:_ipy(?:intervu)?[\\s\\S]*)?$")

// stripIPyIntervuTail removes complete _ipyintervu sync blocks from assistant text.
func stripIPyIntervuTail(content string) string {
	s := ipyintervuFenceStripPattern.ReplaceAllString(content, "")
	return strings.TrimRight(s, " \t\n\r")
}

// clientVisibleAssistantContent is safe to stream/display (complete blocks removed, partial fence held back).
func clientVisibleAssistantContent(accumulated string) string {
	stripped := stripIPyIntervuTail(accumulated)
	if loc := partialIPyIntervuFence.FindStringIndex(stripped); loc != nil {
		matched := stripped[loc[0]:]
		if strings.Count(matched, "```") < 2 {
			stripped = strings.TrimRight(stripped[:loc[0]], " \t\n\r")
		}
	}
	return stripped
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
	assistant    string
	sentToClient bool
	failurePoint string // upstream_read, client_write, none
	err          error
}

func relaySSEFiltered(dst io.Writer, src io.Reader) relaySSEResult {
	return relaySSEFilteredWithBuffer(dst, src, nil)
}

func relaySSEFilteredWithBuffer(dst io.Writer, src io.Reader, shared *sharedTurnStream) relaySSEResult {
	stopKeepalive := startSSERelayKeepalive(dst)
	defer stopKeepalive()

	var accumulated strings.Builder
	var sentVisible int
	var sentToClient bool
	clientGone := false
	buf := make([]byte, 32*1024)
	remainder := ""

	flushPart := func(part string) error {
		delta := parseSSEAssistantDelta(part)
		outPart := part
		if delta != "" {
			accumulated.WriteString(delta)
			visible := clientVisibleAssistantContent(accumulated.String())
			newDelta := visible[sentVisible:]
			sentVisible = len(visible)
			if newDelta == "" {
				return nil
			}
			outPart = rewriteSSEContentDelta(part, newDelta)
		}
		if shared != nil {
			shared.appendChunk(outPart, accumulated.String(), clientVisibleAssistantContent(accumulated.String()))
		}
		if clientGone {
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
				return relaySSEResult{assistant: accumulated.String(), sentToClient: sentToClient, failurePoint: "none"}
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
