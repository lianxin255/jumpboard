package e2e

import (
	"testing"
)

// chatPorts are the ports the chat e2e's drafts listen on, off the ranges
// the app's own tests and the machine's real services use.
var chatPorts = [2]int{19105, 19106}

// chatReply is what the fake model answers the chat page with: words
// around one fenced json block, the shape the chat system prompt asks
// for. The words must survive on the page and the drafts must come out
// of the block, so both halves of the reply are checked by one flow.
func chatReply() string {
	return "两条映射如下：\n\n```json\n{\"drafts\":[" +
		`{"name":"内网 MySQL","protocol":"tcp","listenPort":` + itoa(chatPorts[0]) + `,"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL 到 ` + itoa(chatPorts[0]) + `"},` +
		`{"name":"内网 Redis","protocol":"tcp","listenPort":` + itoa(chatPorts[1]) + `,"upstreamHost":"192.168.1.11","upstreamPort":6379,"explanation":"Redis 到 ` + itoa(chatPorts[1]) + `"}]}` +
		"\n```\n\n导入前可再调整端口。"
}

// TestAIChatPageGuidesToSettings checks the page a user without AI lands
// on: it names what is missing and its one button leads to the settings,
// the same gate the panel's button hides behind.
func TestAIChatPageGuidesToSettings(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("还没有配置 AI 服务") {
		t.Fatalf("the unconfigured page should say so: %v", tt.Texts())
	}
	if err := tt.Click("去设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("接口地址") {
		t.Fatalf("去设置 should land on the settings page: %v", tt.Texts())
	}
}

// TestAIChatImportsDraftsIntoWorkspace walks the chat page end to end:
// the sidebar leads there, a message goes out, the reply's words and
// drafts both show, the drafts import behind their confirmation into the
// current scene as unapplied mappings, and the conversation keeps its
// history for the next round.
func TestAIChatImportsDraftsIntoWorkspace(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, chatReply())
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "把内网 MySQL 和 Redis 分别暴露到 "+itoa(chatPorts[0])+"/"+itoa(chatPorts[1]))
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the reply and its preview", func() bool {
		return tt.HasText("两条映射如下") && tt.HasText("配置预览（2 条）") &&
			tt.HasText("内网 MySQL") && tt.HasText("导入到当前场景")
	})
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Fatalf("talking must store nothing: %+v", list)
	}

	if err := tt.Click("导入到当前场景"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("把 2 条草稿写入当前场景") {
		t.Fatalf("the import should ask first: %v", tt.Texts())
	}
	confirm(t, tt, "导入")

	ws := a.ActiveWorkspace()
	waitFor(t, tt, "the mappings stored", func() bool {
		list, _ := a.Store.ListMappings(ws)
		return len(list) == 2
	})
	list, _ := a.Store.ListMappings(ws)
	if list[0].Name != "内网 MySQL" || list[0].ListenPort != chatPorts[0] {
		t.Fatalf("stored wrongly: %+v", list[0])
	}
	if !a.Dirty() {
		t.Error("an import marks the configuration dirty, like a save")
	}
	// Imported is not applied: nothing listens until the user says so.
	for _, m := range list {
		if listening, _ := a.RuleState(m.ID); listening {
			t.Errorf("「%s」must not start forwarding on its own", m.Name)
		}
	}
	// The preview is gone with its drafts, and the conversation stays.
	tt.Frame()
	if tt.HasText("配置预览") {
		t.Errorf("the preview should leave with its drafts: %v", tt.Texts())
	}
	if !tt.HasText("两条映射如下") {
		t.Errorf("the conversation should keep its turns: %v", tt.Texts())
	}
}

// TestAIChatComposerStaysOnScreen holds the composer where it belongs:
// the conversation scrolls inside its own bounded panel and the field with
// the send button ride one reserved row under it, so nothing the
// conversation grows can push them off the bottom of the window — not even
// at the app's minimum size. The regression this pins: the composer's row
// was once unwrapped, so the field stretched full width and the send
// button stacked under it, flush against the window's bottom edge.
func TestAIChatComposerStaysOnScreen(t *testing.T) {
	for _, sz := range [][2]int{{1280, 800}, {960, 640}} {
		w, h := sz[0], sz[1]
		a := testApp(t)
		srv := fakeOpenAI(t, chatReply())
		a.SetAI(srv.URL, "chat-model")
		tt := tester(t, a, w, h)

		if err := tt.Click("AI 对话"); err != nil {
			t.Fatal(err)
		}
		fillField(t, tt, "消息", "把内网 MySQL 和 Redis 分别暴露到 "+itoa(chatPorts[0])+"/"+itoa(chatPorts[1]))
		if err := tt.Click("发送"); err != nil {
			t.Fatal(err)
		}
		// The longest page: a reply that carries drafts, so the list has
		// everything a conversation can pile onto it.
		waitFor(t, tt, "the reply and its preview", func() bool {
			return tt.HasText("配置预览（2 条）") && tt.HasText("导入到当前场景")
		})
		tt.Frame()

		in, ok := tt.Find("消息")
		if !ok {
			t.Fatalf("%dx%d: the composer's field is missing", w, h)
		}
		send, ok := tt.Find("发送")
		if !ok {
			t.Fatalf("%dx%d: the send button is missing", w, h)
		}
		pad := float32(20) // the page's own bottom padding
		if bottom := in.Y + in.H; bottom > float32(h)-pad+0.5 {
			t.Errorf("%dx%d: the field is pushed off screen: bottom %.1f > %.1f", w, h, bottom, float32(h)-pad)
		}
		if bottom := send.Y + send.H + 8; bottom > float32(h)-pad+0.5 {
			// The label Find returns is the button's text; the chrome
			// carries about 8 DIPs of padding under it.
			t.Errorf("%dx%d: the send button is pushed off screen: bottom %.1f > %.1f", w, h, bottom, float32(h)-pad)
		}
		// Side by side: the button starts after the field ends and rides
		// its band, rather than stretching below it.
		if send.X < in.X+in.W {
			t.Errorf("%dx%d: the send button should sit beside the field, not under it: field=%+v send=%+v", w, h, in, send)
		}
		if send.Y >= in.Y+in.H || send.Y+send.H <= in.Y {
			t.Errorf("%dx%d: the send button should share the field's band: field=%+v send=%+v", w, h, in, send)
		}
		a.Close()
	}
}
