package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// mysqlDraft is one complete draft, the table cases' shared item.
const mysqlDraft = `{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,` +
	`"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL 到 9000"}`

// TestDraftMultiParsesDrafts checks the asked-for shape: one draft per
// service, in the order the sentence named them, each still passing the
// ordinary validation.
func TestDraftMultiParsesDrafts(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(chatReply(`{"drafts":[` + mysqlDraft + `,` +
			`{"name":"内网 Redis","protocol":"tcp","listenPort":9001,` +
			`"upstreamHost":"192.168.1.11","upstreamPort":6379,"explanation":"Redis 到 9001"}]}`))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "t"})
	drafts, err := c.DraftMulti(context.Background(), "把内网 MySQL 和 Redis 分别暴露到 9000/9001", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 {
		t.Fatalf("drafts = %d, want 2", len(drafts))
	}
	if drafts[0].Name != "内网 MySQL" || drafts[0].ListenPort != 9000 {
		t.Errorf("first draft: %+v", drafts[0])
	}
	if drafts[1].Name != "内网 Redis" || drafts[1].UpstreamPort != 6379 {
		t.Errorf("second draft: %+v", drafts[1])
	}
	for _, d := range drafts {
		if _, _, err := d.Mapping(); err != nil {
			t.Errorf("draft %q does not survive Mapping: %v", d.Name, err)
		}
	}
}

// TestDraftMultiReplyShapes checks the shapes a live model answers with
// besides the asked-for wrapper: a bare array, a fenced array, a wrapper
// under another name, snake_case field aliases, and — a model that
// answered the single-draft prompt — one bare draft object, which is
// accepted as a list of one.
func TestDraftMultiReplyShapes(t *testing.T) {
	cases := []struct{ name, reply string }{
		{"bare array", `[` + mysqlDraft + `]`},
		{"fenced array", "```json\n[" + mysqlDraft + "]\n```"},
		{"wrapper alias", `{"rules":[` + mysqlDraft + `]}`},
		{"field aliases", `{"drafts":[{` +
			`"name":"内网 MySQL","protocol":"tcp","listen_port":"9000",` +
			`"upstreamIp":"192.168.1.10","target_port":3306}]}`},
		{"single object", mysqlDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write(chatReply(tc.reply))
			})
			c := New(Settings{BaseURL: srv.URL, Model: "t"})
			drafts, err := c.DraftMulti(context.Background(), "x", 5)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(drafts) != 1 {
				t.Fatalf("%s: %d drafts, want 1", tc.name, len(drafts))
			}
			d := drafts[0]
			if d.Name != "内网 MySQL" || d.ListenPort != 9000 ||
				d.UpstreamHost != "192.168.1.10" || d.UpstreamPort != 3306 {
				t.Fatalf("%s: %+v", tc.name, d)
			}
		})
	}
}

// TestDraftMultiTruncatesToFive checks the cap from both sides: a reply
// with more than five keeps the first five, and an n above the cap is the
// cap itself.
func TestDraftMultiTruncatesToFive(t *testing.T) {
	items := make([]string, 7)
	for i := range items {
		items[i] = fmt.Sprintf(`{"name":"服务 %d","protocol":"tcp","listenPort":%d,`+
			`"upstreamHost":"10.0.0.1","upstreamPort":3306}`, i, 9100+i)
	}
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(chatReply(`{"drafts":[` + strings.Join(items, ",") + `]}`))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "t"})
	ctx := context.Background()

	drafts, err := c.DraftMulti(ctx, "x", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 5 || drafts[0].Name != "服务 0" || drafts[4].Name != "服务 4" {
		t.Fatalf("kept %d drafts, want the first five: %+v", len(drafts), drafts)
	}
	// An n past the cap is the cap; a non-positive n is a caller bug, not
	// a license for unlimited drafts.
	for _, n := range []int{9, 0, -1} {
		drafts, err := c.DraftMulti(ctx, "x", n)
		if err != nil || len(drafts) != 5 {
			t.Fatalf("n=%d: %d drafts, err %v", n, len(drafts), err)
		}
	}
}

// TestDraftMultiDropsBrokenItems checks that one broken item drops alone:
// an empty object and a bare string do not void the good draft beside
// them.
func TestDraftMultiDropsBrokenItems(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(chatReply(`{"drafts":[` + mysqlDraft + `,{},"不是对象"]}`))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "t"})
	drafts, err := c.DraftMulti(context.Background(), "x", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].Name != "内网 MySQL" {
		t.Fatalf("drafts: %+v", drafts)
	}
}

// TestDraftMultiRefusesUnusableReplies checks the errors the panel shows:
// an empty list, a reply that is not JSON, and an object that holds
// neither drafts nor a usable single draft.
func TestDraftMultiRefusesUnusableReplies(t *testing.T) {
	cases := []struct{ name, reply string }{
		{"empty list", `{"drafts":[]}`},
		{"not json", `好的，这是您要的规则`},
		{"neither drafts nor a draft", `{"summary":"无法理解"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write(chatReply(tc.reply))
			})
			c := New(Settings{BaseURL: srv.URL, Model: "t"})
			if _, err := c.DraftMulti(context.Background(), "x", 5); err == nil {
				t.Fatalf("%s should be an error", tc.name)
			}
		})
	}
}

// TestForModel checks the per-purpose rename: another name reaches the
// wire, an empty name or the client's own is the client itself, and the
// original client still names its own model after a twin went out.
func TestForModel(t *testing.T) {
	var got []string
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req.Model)
		w.Write(chatReply(mysqlDraft))
	})
	base := New(Settings{BaseURL: srv.URL, Model: "main", APIKey: "sk-test"})
	if base.ForModel("") != base {
		t.Error("an empty name is the client itself")
	}
	if base.ForModel("main") != base {
		t.Error("the client's own name is the client itself")
	}
	ctx := context.Background()
	if _, err := base.ForModel("diag-model").Draft(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Draft(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "diag-model" || got[1] != "main" {
		t.Fatalf("models on the wire: %v", got)
	}
}
