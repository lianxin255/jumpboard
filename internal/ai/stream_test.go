package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestPostChatStreamDeliversDeltas checks the streaming request and reader:
// the wire carries stream:true and no response_format, and a reply that
// arrives in several writes — one of them cutting a line in half — is
// handed over fragment by fragment, in order, until [DONE].
func TestPostChatStreamDeliversDeltas(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("request body: %v", err)
		}
		if req["stream"] != true {
			t.Errorf("stream = %v, want true", req["stream"])
		}
		if _, ok := req["response_format"]; ok {
			t.Error("a streaming request should not ask for a JSON response format")
		}
		if msgs, ok := req["messages"].([]any); !ok || len(msgs) != 2 {
			t.Errorf("messages = %v, want system+user", req["messages"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"把内网 \"}}]}\n\n"))
		fl.Flush()
		// One event split across two writes, in the middle of a line.
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"My"))
		fl.Flush()
		w.Write([]byte("SQL 暴露到 9000\"}}]}\n\n"))
		fl.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
		fl.Flush()
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test", APIKey: "sk-test"})

	var got []string
	text, err := c.postChatStream(context.Background(), "s", "u", func(frag string) { got = append(got, frag) })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"把内网 ", "MySQL 暴露到 9000"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("fragments = %q, want %q", got, want)
	}
	if text != "把内网 MySQL 暴露到 9000" {
		t.Errorf("text = %q", text)
	}
}

// TestPostChatAutoUsesPlainJSONResponse checks the gateway that ignores
// stream:true and answers a normal JSON completion: the auto path decodes
// that first response itself instead of repeating the model request.
func TestPostChatAutoUsesPlainJSONResponse(t *testing.T) {
	calls := 0
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatReply("```json\n{\"ok\":true}\n```"))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test"})

	// The low-level stream reader keeps its old, narrow contract; the auto
	// path owns compatibility with a non-SSE gateway.
	if text, err := c.postChatStream(context.Background(), "s", "u", nil); err != nil || text != "" {
		t.Fatalf("the stream attempt should end empty without an error: text=%q err=%v", text, err)
	}
	beforeAuto := calls
	deltas := 0
	text, err := c.postChatAuto(context.Background(), "s", "u", func(string) { deltas++ })
	if err != nil {
		t.Fatal(err)
	}
	if calls-beforeAuto != 1 {
		t.Fatalf("auto path made %d requests, want exactly 1", calls-beforeAuto)
	}
	// The fence is gone just as it is on the ordinary one-shot JSON path.
	if text != `{"ok":true}` {
		t.Errorf("text = %q, want the extracted JSON", text)
	}
	if deltas != 0 {
		t.Errorf("deltas = %d, want 0 for a non-SSE response", deltas)
	}
}

// TestPostChatAutoFallsBackOnHTTPError checks a 500 on the streaming
// request: postChatStream surfaces the gateway's error message, and
// postChatAuto still succeeds because nothing had been delivered yet.
func TestPostChatAutoFallsBackOnHTTPError(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("request body: %v", err)
		}
		if req["stream"] == true {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "streaming not enabled"},
			})
			return
		}
		w.Write(chatReply(`{"recovered":true}`))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test"})

	_, err := c.postChatStream(context.Background(), "s", "u", nil)
	if err == nil || !strings.Contains(err.Error(), "streaming not enabled") {
		t.Fatalf("the stream error should carry the gateway's message: %v", err)
	}

	// A nil onDelta must survive the whole path too.
	text, err := c.postChatAuto(context.Background(), "s", "u", nil)
	if err != nil {
		t.Fatal(err)
	}
	if text != `{"recovered":true}` {
		t.Errorf("text = %q", text)
	}
}

// TestPostChatAutoKeepsPartialWhenStreamDrops checks a stream that delivers
// two fragments and then dies: the answer has started arriving, so the
// partial text is returned with the error instead of starting over.
func TestPostChatAutoKeepsPartialWhenStreamDrops(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		// More bytes promised than written: the client's read ends with
		// an unexpected EOF once the handler returns and the connection
		// closes short.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", "4096")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"stream\"}}]}\n\n"))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test"})

	text, err := c.postChatAuto(context.Background(), "s", "u", func(string) {})
	if err == nil {
		t.Fatal("a dropped stream should be an error")
	}
	if text != "Hello stream" {
		t.Errorf("text = %q, want the partial \"Hello stream\"", text)
	}
}

// TestPostChatStreamToleratesChunkShapes checks the shapes a gateway may
// send inside an otherwise correct stream: comments, foreign fields, data:
// without the space, empty deltas, events without choices, usage-only
// trailers, and events after [DONE] — none of them text, and none of them
// fatal. (A gateway that puts message.content where delta.content belongs
// is tolerated the same way, but its text is lost: the library's reader
// knows the one slot, and widening it would mean hand-rolling the wire
// this package no longer owns.)
func TestPostChatStreamToleratesChunkShapes(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
			": keep-alive\n\n" +
			"event: rename\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
			"data:{\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"total_tokens\":9}}\n\n" +
			"data: {\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\" ignored\"}}]}\n\n" +
			"data: [DONE]\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"after done\"}}]}\n\n"))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test"})

	var got []string
	text, err := c.postChatStream(context.Background(), "s", "u", func(frag string) { got = append(got, frag) })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello world" {
		t.Errorf("text = %q, want \"Hello world\"", text)
	}
	if len(got) != 2 || got[0] != "Hello" || got[1] != " world" {
		t.Errorf("fragments = %q, want [Hello  world]", got)
	}

	// A nil onDelta is safe with real fragments too.
	if text, err := c.postChatStream(context.Background(), "s", "u", nil); err != nil || text != "Hello world" {
		t.Errorf("nil onDelta: text = %q, err = %v", text, err)
	}
}

// sseReply writes a complete streaming reply: every fragment becomes one
// data event, each flushed, ending with [DONE].
func sseReply(w http.ResponseWriter, frags ...string) []byte {
	w.Header().Set("Content-Type", "text/event-stream")
	fl := w.(http.Flusher)
	fl.Flush()
	var b bytes.Buffer
	b.WriteString("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
	for _, f := range frags {
		b.WriteString("data: {\"choices\":[{\"delta\":{\"content\":" + string(mustJSON(f)) + "}}]}\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	fl.Flush()
	return b.Bytes()
}

// sseReplyThenBreak writes one fragment and then cuts the connection
// without [DONE] or a clean end — a socket dropped mid-reply.
func sseReplyThenBreak(w http.ResponseWriter, frag string) []byte {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + string(mustJSON(frag)) + "}}]}\n\n"))
	w.(http.Flusher).Flush()
	// A socket dropped mid-reply: the connection closes without [DONE] and
	// without a clean end, so the reader sees an unexpected EOF.
	panic(http.ErrAbortHandler)
}

// mustJSON encodes one string as a JSON string literal for the helpers.
func mustJSON(s string) []byte {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return b
}
