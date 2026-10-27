package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// streamRequest is the streaming twin of chatRequest: the same model,
// messages and low temperature, but "stream": true and no response_format,
// so a capable gateway answers server-sent events instead of one JSON
// object. A struct of its own keeps the one-shot chatRequest — and every
// feature already reading it — untouched.
type streamRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

// streamChunk narrows one SSE data event to what the client reads. A
// well-behaved gateway sends choices[0].delta.content; some send
// message.content in that slot instead, and either object may be absent on
// role-only, empty or terminal chunks, so both are pointers and whichever
// carries text wins.
type streamChunk struct {
	Choices []struct {
		Delta   *chatMessage `json:"delta"`
		Message *chatMessage `json:"message"`
	} `json:"choices"`
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
// text. Which of the two happened is what postChatAuto's fallback turns on.
func (c *Client) postChatStream(ctx context.Context, system, user string, onDelta func(string)) (string, error) {
	if !c.settings.Ready() {
		return "", errors.New("还没有配置 AI 服务")
	}
	body, err := json.Marshal(streamRequest{
		Model:  c.settings.Model,
		Stream: true,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	url := normalizeBase(c.settings.BaseURL) + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.settings.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接 AI 服务失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", fmt.Errorf("AI 服务返回 %d", resp.StatusCode)
		}
		var r chatResponse
		if json.Unmarshal(data, &r) == nil && r.Error != nil && r.Error.Message != "" {
			return "", fmt.Errorf("AI 服务返回 %d：%s", resp.StatusCode, r.Error.Message)
		}
		return "", fmt.Errorf("AI 服务返回 %d", resp.StatusCode)
	}
	if mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil || !strings.EqualFold(mt, "text/event-stream") {
		// A gateway that answers anything but an event stream — most often
		// the normal JSON completion of one that ignores stream:true — did
		// not stream. Not an error worth showing on its own: the caller
		// falls back to the one-shot path.
		return "", errors.New("AI 服务没有按流式响应")
	}

	// Lines, not the whole body: the point is to hand each fragment over
	// as it arrives. The 1MiB cap matches the one-shot read limit, since
	// a single event still carries whatever the model wrote in one piece.
	var text strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		payload, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			// Blank lines between events, comments, keep-alives and other
			// fields carry no text.
			continue
		}
		payload = strings.TrimPrefix(payload, " ")
		if payload == "[DONE]" {
			return text.String(), nil
		}
		var chunk streamChunk
		if json.Unmarshal([]byte(payload), &chunk) != nil {
			continue // a malformed chunk is skipped, not fatal
		}
		var frag string
		if len(chunk.Choices) > 0 {
			if d := chunk.Choices[0].Delta; d != nil {
				frag = d.Content
			}
			if frag == "" && chunk.Choices[0].Message != nil {
				frag = chunk.Choices[0].Message.Content
			}
		}
		if frag == "" {
			// Role-only and usage-only chunks arrive without text; they
			// are progress, not content.
			continue
		}
		text.WriteString(frag)
		if onDelta != nil {
			onDelta(frag)
		}
	}
	if err := sc.Err(); err != nil {
		return text.String(), fmt.Errorf("AI 流式响应中断：%w", err)
	}
	// A clean end without [DONE]: some gateways close the stream instead
	// of terminating it, and the body framing has already proved complete.
	return text.String(), nil
}

// postChatAuto streams the reply when the gateway can and answers in one
// request when it cannot, so a feature can render text as it arrives
// without knowing what the endpoint supports.
//
// The fallback rule: if the streaming attempt fails — request error,
// non-200, a body that is not an event stream, an empty stream — before a
// single fragment has been handed to onDelta, nothing has been shown yet,
// and the existing one-shot postChat is retried once and its result
// returned. After the first delivered fragment the answer has started
// arriving: a mid-stream failure returns the partial text with the error
// rather than restarting, so the caller can show what arrived instead of
// waiting twice. Check err before using the text in that case.
func (c *Client) postChatAuto(ctx context.Context, system, user string, onDelta func(string)) (string, error) {
	var started bool
	text, err := c.postChatStream(ctx, system, user, func(frag string) {
		started = true
		if onDelta != nil {
			onDelta(frag)
		}
	})
	if err == nil && text != "" {
		return text, nil
	}
	if !started {
		return c.postChat(ctx, system, user)
	}
	return text, err
}
