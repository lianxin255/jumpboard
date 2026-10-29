package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// plainCompletionCapture holds the one successful JSON body a gateway sent
// to a stream request. go-openai's stream reader owns SSE, but exposes no way
// to recover a non-SSE body after it reads that body as an empty stream.
type plainCompletionCapture struct {
	body []byte
}

type plainCompletionContextKey struct{}

// plainCompletionTransport buffers only successful JSON responses to the
// auto-stream requests marked with a capture. Event streams and error bodies
// pass through untouched, preserving go-openai's streaming and error behavior.
type plainCompletionTransport struct {
	base http.RoundTripper
}

func (t plainCompletionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	capture, _ := req.Context().Value(plainCompletionContextKey{}).(*plainCompletionCapture)
	mediaType := resp.Header.Get("Content-Type")
	if capture == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 ||
		!strings.HasPrefix(strings.ToLower(mediaType), "application/json") {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	capture.body = body
	return resp, nil
}

// postChatStream sends the same chat completion as postChat — same URL,
// headers, model and temperature 0.2 — with "stream": true and no JSON
// response format, and collects the reply from the server-sent events
// instead of one body. Every non-empty fragment is appended and passed to
// onDelta in arrival order; onDelta may be nil.
//
// On failure it returns the text received so far together with the error,
// never discarding what already arrived: a request that fails before any
// event yields "", and a stream that breaks mid-way yields the partial
// text. Which of the two happened is what postChatAuto's fallback turns
// on. The reader is the library's, which also means a gateway that
// answers one plain JSON body despite stream:true is not an error here —
// it reads as an empty stream, "" and nil, and the same fallback in
// postChatAuto is what recovers it.
func (c *Client) postChatStream(ctx context.Context, system, user string, onDelta func(string)) (string, error) {
	return c.postChatStreamMsgs(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: system},
		{Role: openai.ChatMessageRoleUser, Content: user},
	}, onDelta)
}

// postChatStreamMsgs is postChatStream over a whole message list, the way
// postChatMsgs is postChat's: the conversation in chat.go streams its
// history through this one.
//
// The reader delivers each data event's delta content as one Recv; io.EOF
// is the stream's end, whether the gateway sent [DONE] or closed cleanly
// without it. A role-only or usage-only event carries no content and is
// passed over, as is one after the stream's end.
func (c *Client) postChatStreamMsgs(ctx context.Context, msgs []openai.ChatCompletionMessage, onDelta func(string)) (string, error) {
	if !c.settings.Ready() {
		return "", errors.New("还没有配置 AI 服务")
	}
	stream, err := c.oa.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       c.settings.Model,
		Messages:    msgs,
		Temperature: chatTemperature(c.settings.Model),
	})
	if err != nil {
		return "", mapOpenAIError(err)
	}
	defer stream.Close()

	var text strings.Builder
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return text.String(), nil
		}
		if err != nil {
			var apiErr *openai.APIError
			if errors.As(err, &apiErr) {
				// The stream carried an error event; its words are worth
				// more than the interruption itself.
				return text.String(), mapOpenAIError(err)
			}
			return text.String(), fmt.Errorf("AI 流式响应中断：%w", err)
		}
		if len(resp.Choices) == 0 {
			continue
		}
		frag := resp.Choices[0].Delta.Content
		if frag == "" {
			continue
		}
		text.WriteString(frag)
		if onDelta != nil {
			onDelta(frag)
		}
	}
}

// postChatAuto streams the reply when the gateway can and answers in one
// request when it cannot, so a feature can render text as it arrives
// without knowing what the endpoint supports.
//
// The fallback rule: if the streaming attempt fails — request error,
// non-200, an empty stream, a body that was not one — before a single
// fragment has been handed to onDelta, nothing has been shown yet, and
// the existing one-shot postChat is retried once and its result returned.
// A successful plain JSON completion is the exception: its first body is
// decoded directly, so a gateway that ignores stream:true costs one POST.
// After the first delivered fragment the answer has started arriving: a
// mid-stream failure returns the partial text with the error rather than
// restarting, so the caller can show what arrived instead of
// waiting twice. Check err before using the text in that case.
func (c *Client) postChatAuto(ctx context.Context, system, user string, onDelta func(string)) (string, error) {
	return c.postChatAutoMsgs(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: system},
		{Role: openai.ChatMessageRoleUser, Content: user},
	}, true, onDelta)
}

// postChatAutoMsgs is postChatAuto over a whole message list, with the
// same fallback rule: a streaming attempt that fails before the first
// fragment is retried once through the one-shot path; after the first
// fragment the partial text travels with the error. jsonFormat says what
// the one-shot fallback wants, the same thing postChatMsgs' flag says —
// true for the one-shot features' JSON object, false for the
// conversation's prose. The conversation in chat.go is the false caller.
func (c *Client) postChatAutoMsgs(ctx context.Context, msgs []openai.ChatCompletionMessage, jsonFormat bool, onDelta func(string)) (string, error) {
	capture := new(plainCompletionCapture)
	ctx = context.WithValue(ctx, plainCompletionContextKey{}, capture)
	var started bool
	text, err := c.postChatStreamMsgs(ctx, msgs, func(frag string) {
		started = true
		if onDelta != nil {
			onDelta(frag)
		}
	})
	if err == nil && text != "" {
		return text, nil
	}
	if !started && err == nil && len(capture.body) > 0 {
		// The gateway ignored stream:true but already returned the complete
		// completion. Use that first response; a second POST would repeat a
		// non-idempotent model request just to decode the body differently.
		var resp openai.ChatCompletionResponse
		if err := json.Unmarshal(capture.body, &resp); err != nil {
			return "", mapOpenAIError(err)
		}
		return chatCompletionContent(resp, jsonFormat)
	}
	if !started {
		return c.postChatMsgs(ctx, msgs, jsonFormat)
	}
	return text, err
}
