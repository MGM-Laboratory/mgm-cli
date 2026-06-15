//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// capturedRequest records what the CLI sent to the broker's /v1/messages
// endpoint, so tests can assert on the full auth → broker request pipeline.
type capturedRequest struct {
	Method           string
	Path             string
	APIKey           string // x-api-key header (the injected Megumi credential)
	Authorization    string // must be empty: the credential hook strips it
	Effort           string // x-megumi-effort header
	Project          string // x-megumi-project header
	AnthropicVer     string // anthropic-version header
	Body             map[string]any
	RawBody          string // the full request body as sent (for substring asserts)
	UserPromptInBody string
}

// mockBroker is an httptest server that emulates the Megumi backend: the
// identity endpoint (/api/v1/me) and an Anthropic-compatible streaming
// /v1/messages endpoint that returns a deterministic SSE response.
type mockBroker struct {
	*httptest.Server

	mu       sync.Mutex
	requests []capturedRequest

	// replyText is the assistant text streamed back for /v1/messages.
	replyText string
}

func newMockBroker(t *testing.T, replyText string) *mockBroker {
	t.Helper()
	b := &mockBroker{replyText: replyText}

	mux := http.NewServeMux()

	// Identity endpoint used by the API-code verification handshake.
	mux.HandleFunc("/api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sub":    "e2e-subject",
			"email":  "e2e@labmgm.org",
			"role":   "member",
			"method": "api_code",
		})
	})

	// Anthropic-compatible streaming messages endpoint.
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		b.capture(r)
		b.streamAnthropicSSE(w)
	})

	// Catch-all: record and 404 so an unexpected path surfaces in assertions.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b.capture(r)
		http.NotFound(w, r)
	})

	b.Server = httptest.NewServer(mux)
	t.Cleanup(b.Server.Close)
	return b
}

func (b *mockBroker) capture(r *http.Request) {
	rec := capturedRequest{
		Method:        r.Method,
		Path:          r.URL.Path,
		APIKey:        r.Header.Get("x-api-key"),
		Authorization: r.Header.Get("Authorization"),
		Effort:        r.Header.Get("x-megumi-effort"),
		Project:       r.Header.Get("x-megumi-project"),
		AnthropicVer:  r.Header.Get("anthropic-version"),
	}
	if body, err := io.ReadAll(r.Body); err == nil && len(body) > 0 {
		rec.RawBody = string(body)
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) == nil {
			rec.Body = parsed
			rec.UserPromptInBody = extractUserText(parsed)
		}
	}
	b.mu.Lock()
	b.requests = append(b.requests, rec)
	b.mu.Unlock()
}

// messageRequests returns only the captured POSTs to /v1/messages.
func (b *mockBroker) messageRequests() []capturedRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []capturedRequest
	for _, r := range b.requests {
		if r.Path == "/v1/messages" {
			out = append(out, r)
		}
	}
	return out
}

// streamAnthropicSSE writes a minimal but valid Anthropic Messages streaming
// response: message_start → content_block_start → text_delta → stops. This is
// the stable wire format the Anthropic SDK the agent uses expects.
func (b *mockBroker) streamAnthropicSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	send := func(event string, data map[string]any) {
		payload, _ := json.Marshal(data)
		_, _ = io.WriteString(w, "event: "+event+"\n")
		_, _ = io.WriteString(w, "data: "+string(payload)+"\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}

	send("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            "msg_e2e_1",
			"type":          "message",
			"role":          "assistant",
			"model":         "gumi",
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 5, "output_tokens": 0},
		},
	})
	send("ping", map[string]any{"type": "ping"})
	send("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
	send("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]any{"type": "text_delta", "text": b.replyText},
	})
	send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	send("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": 8},
	})
	send("message_stop", map[string]any{"type": "message_stop"})
}

// extractUserText digs the first user text out of an Anthropic request body
// ({"messages":[{"role":"user","content":[{"type":"text","text":"..."}]}]} or
// the string-content shorthand).
func extractUserText(body map[string]any) string {
	msgs, ok := body["messages"].([]any)
	if !ok {
		return ""
	}
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok || mm["role"] != "user" {
			continue
		}
		switch c := mm["content"].(type) {
		case string:
			return c
		case []any:
			for _, part := range c {
				if pm, ok := part.(map[string]any); ok {
					if txt, ok := pm["text"].(string); ok && txt != "" {
						return txt
					}
				}
			}
		}
	}
	return ""
}
