// The conversation the AI chat page holds: whole-turn history in, one
// reply out, streaming when the gateway can. The reply is prose that may
// carry mapping drafts as a fenced JSON block, so this file also reads
// drafts back out of a reply — the same parseDrafts the one-shot paths
// use, pointed at a block instead of a whole answer.
package ai

import (
	"context"
	"errors"
	"strings"
)

// The roles a conversation turn carries. The system role is not one of
// them: wireTurns prepends the system prompt itself, so a turn claiming
// it is dropped rather than let a reply smuggle a new instruction set in.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatTurn is one turn of the conversation, the history the page keeps
// and the request sends.
type ChatTurn struct {
	Role    string
	Content string
}

// chatTurnsMax caps how many turns of the history travel. A conversation
// that outgrows it keeps its head in the window and loses its oldest
// turns on the wire only; the page still shows them all.
const chatTurnsMax = 20

// chatTurnCut caps one turn's content by runes, the way Truncate cuts
// logs: one giant pasted reply must not crowd the newest question out of
// the request.
const chatTurnCut = 8000

// chatSystemPrompt tells the model what the conversation is for and what
// a configuration turn looks like. The guardrail at the end matters as
// much as the task: drafts are proposals the user imports by hand, and a
// model that says "已生效" is lying about a thing it cannot do. The fence
// in rule 2 is spliced in, a triple backtick a backquoted string cannot
// hold.
const chatFence = "```json"

const chatSystemPrompt = `You are JumpBoard's port mapping assistant. JumpBoard
forwards ports on this machine to services on the internal network; one
mapping is a protocol (tcp/udp/http/https), a listen port on this machine,
and an upstream host and port on the network.

This is a conversation. Answer the user's questions about ports,
forwarding and the app in Chinese, in concise markdown, using what the
earlier turns already said rather than asking for it again.

When the user wants mappings proposed or changed, do two things in one
reply:
1. A short Chinese explanation of what you understood, including anything
   missing or uncertain.
2. A fenced ` + chatFence + ` block, the only machine-readable thing in the reply,
   exactly this shape:
   {"drafts":[{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,"serverNames":"","listenPath":"/","upstreamHost":"192.168.1.10","upstreamPort":3306,"proxyMode":"standard","extraDirectives":"","explanation":"把内网 MySQL 暴露到 9000"}]}

Draft field rules:
- One draft per distinct service named. protocol "tcp" for databases,
  SSH, RDP and raw ports, "udp" for DNS and datagram protocols,
  "http"/"https" only for web services.
- listenPort is the port on THIS machine; upstreamPort is the service's.
  If the user does not say a listen port, choose a sensible one above
  8000.
- Never invent an IP address the user did not give. If the upstream is
  missing, use "" and say so in that draft's explanation.
- extraDirectives: only plain directives such as "proxy_timeout 1h;".
  Never write a server block, a brace, or anything that could contain one.
- Each draft's explanation is in Chinese.

Drafts are proposals only. You cannot apply, start or change anything:
the user reviews them and imports them by hand. Never claim a rule is
running.`

// wireTurns builds the message list one request sends: the system prompt,
// then the history — trimmed to the newest chatTurnsMax turns, each cut
// to chatTurnsCut runes, and re-cut to start on a user turn, since a
// conversation that opens on the assistant's half of a pair reads as a
// broken transcript to gateways that check.
func wireTurns(turns []ChatTurn) []chatMessage {
	out := make([]chatMessage, 0, len(turns)+1)
	out = append(out, chatMessage{Role: "system", Content: chatSystemPrompt})
	for _, t := range turns {
		if t.Role != RoleUser && t.Role != RoleAssistant {
			continue
		}
		content := strings.TrimSpace(t.Content)
		if content == "" {
			continue
		}
		out = append(out, chatMessage{Role: t.Role, Content: Truncate(content, chatTurnCut)})
	}
	if len(out) > chatTurnsMax+1 {
		// The system prompt sits outside the window: the cap is on
		// turns, so it is peeled off first and put back after.
		window := out[1:]
		window = window[len(window)-chatTurnsMax:]
		for len(window) > 0 && window[0].Role != RoleUser {
			window = window[1:]
		}
		out = append([]chatMessage{out[0]}, window...)
	}
	return out
}

// Chat sends the conversation and returns the assistant's reply, waiting
// for the whole answer.
func (c *Client) Chat(ctx context.Context, turns []ChatTurn) (string, error) {
	return c.ChatStream(ctx, turns, nil)
}

// ChatStream sends the conversation and hands the reply over as it
// arrives. onDelta receives every fragment in arrival order and may be
// nil. A gateway that cannot stream answers in one request instead,
// before any fragment is delivered; a stream that drops after the first
// fragment returns the partial text together with the error, the
// postChatAutoMsgs contract.
func (c *Client) ChatStream(ctx context.Context, turns []ChatTurn, onDelta func(string)) (string, error) {
	text, err := c.postChatAutoMsgs(ctx, wireTurns(turns), false, onDelta)
	if err != nil {
		return text, err
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("模型没有回复")
	}
	return text, nil
}

// fencedBlocks returns the contents of the reply's fenced code blocks,
// fence lines stripped. An unclosed fence gives up what it holds rather
// than losing the drafts inside it.
func fencedBlocks(s string) []string {
	var out []string
	var cur []string
	in := false
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
		}
		cur = nil
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if in {
				flush()
			}
			in = !in
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	if in {
		flush()
	}
	return out
}

// draftsIn reads one block, nil when it holds none. A fenced example of
// something else — a command, an unrelated object — parses to nothing and
// stays out of the preview.
func draftsIn(block string) []Draft {
	list, err := parseDrafts(extractJSON(block))
	if err != nil {
		return nil
	}
	if len(list) > maxDrafts {
		list = list[:maxDrafts]
	}
	return list
}

// DraftsFrom scans a chat reply for mapping drafts: each fenced block
// first, then — for a model that skipped the fence — whatever JSON the
// prose embeds, through the same extractJSON the one-shot paths use. A
// reply with no drafts is a normal conversational answer and returns nil,
// not an error. Across all blocks the answer keeps at most maxDrafts
// drafts, the one-shot cap.
func DraftsFrom(reply string) []Draft {
	var out []Draft
	for _, block := range fencedBlocks(reply) {
		out = append(out, draftsIn(block)...)
		if len(out) >= maxDrafts {
			return out[:maxDrafts]
		}
	}
	if len(out) == 0 {
		return draftsIn(reply)
	}
	return out
}

// WithoutDraftBlocks strips the fenced blocks DraftsFrom reads as drafts,
// so the bubble shows the words and the preview cards show the drafts,
// never both. A fence that is not drafts — a command, an example — stays,
// and the blank runs a dropped block leaves behind collapse.
func WithoutDraftBlocks(s string) string {
	var out []string
	var open string
	var cur []string
	in := false
	drop := func(closed bool) {
		if len(draftsIn(strings.Join(cur, "\n"))) == 0 {
			out = append(out, open)
			out = append(out, cur...)
			if closed {
				out = append(out, "```")
			}
		}
		open, cur, in = "", nil, false
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if in {
				drop(true)
			} else {
				in, open = true, line
			}
			continue
		}
		if in {
			cur = append(cur, line)
			continue
		}
		out = append(out, line)
	}
	if in {
		drop(false)
	}
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(joined)
}
