package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

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
// the existing one-shot postChat is retried once and its result
// returned. After the first delivered fragment the answer has started
// arriving: a mid-stream failure returns the partial text with the error
// rather than restarting, so the caller can show what arrived instead of
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
	if !started {
		return c.postChatMsgs(ctx, msgs, jsonFormat)
	}
	return text, err
}
