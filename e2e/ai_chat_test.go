package e2e

import (
	"strings"
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
// the same gate the mappings page's ✨ entry hides behind.
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
		return tt.HasText("两条映射如下") && tt.HasText("规则草稿（2 条）") &&
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
	if tt.HasText("规则草稿") {
		t.Errorf("the preview should leave with its drafts: %v", tt.Texts())
	}
	if !tt.HasText("两条映射如下") {
		t.Errorf("the conversation should keep its turns: %v", tt.Texts())
	}
}

// chatSixReply is what the fake model answers the cap test with: six
// usable drafts in one fenced block, one past what the preview keeps.
func chatSixReply() string {
	names := []string{"一", "二", "三", "四", "五", "六"}
	items := make([]string, len(names))
	for i, name := range names {
		items[i] = `{"name":"服务` + name + `","protocol":"tcp","listenPort":` + itoa(19110+i) +
			`,"upstreamHost":"10.0.0.1","upstreamPort":3306,"explanation":"第` + name + `条"}`
	}
	return "```json\n{\"drafts\":[" + strings.Join(items, ",") + "]}\n```"
}

// TestAIChatCapsDraftsWithNote pins the preview's cap: the sixth usable
// draft never becomes a card, and the note under the head says the cut —
// five cards standing for six rules in silence is the regression.
func TestAIChatCapsDraftsWithNote(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, chatSixReply())
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "把这六个服务都转出去")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "five cards and the cap's note", func() bool {
		return tt.HasText("规则草稿（5 条）") && tt.HasText("仅保留前 5 条")
	})
	for _, want := range []string{"服务一", "服务五"} {
		if !tt.HasText(want) {
			t.Errorf("the kept cards should show %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("服务六") {
		t.Errorf("the sixth draft must not show as a card: %v", tt.Texts())
	}
}

// TestAIChatAllDraftsDroppedShowsNote checks the reply whose drafts were
// all unusable: no preview exists to point at, so the bubble stops
// claiming one and the note rides the conversation as its own line.
func TestAIChatAllDraftsDroppedShowsNote(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, "```json\n{\"drafts\":["+
		`{"name":"缺上游","protocol":"tcp","listenPort":`+itoa(chatPorts[0])+`,"upstreamHost":"","upstreamPort":80,"explanation":"没有地址"},`+
		`{"name":"也缺","protocol":"tcp","listenPort":`+itoa(chatPorts[1])+`,"upstreamHost":"","upstreamPort":80,"explanation":"也没有"}`+
		"]}\n```")
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "转两个服务")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the note as its own line", func() bool {
		return tt.HasText("已丢弃 2 条不完整的草稿")
	})
	// The bubble says the reply held drafts without pointing below itself:
	// there is no preview to see.
	if tt.HasText("见下方预览") {
		t.Errorf("the bubble should not claim a preview that does not exist: %v", tt.Texts())
	}
	if !tt.HasText("（这条回复只有配置草稿）") {
		t.Errorf("the bubble should still say the reply held drafts: %v", tt.Texts())
	}
	if tt.HasText("规则草稿") || tt.HasText("导入到当前场景") {
		t.Errorf("no preview should show without usable drafts: %v", tt.Texts())
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Fatalf("nothing may be stored: %+v", list)
	}
}

// TestAIChatHTTPSDraftUsesDefaultDomainAddress keeps the draft card's local
// address identical to the saved rule row: server name, HTTPS default 443,
// and the default marker rather than a misleading undetermined port.
func TestAIChatHTTPSDraftUsesDefaultDomainAddress(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, "```json\n{\"drafts\":["+
		`{"name":"加密站","protocol":"https","listenPort":0,"serverNames":"git.example.com","upstreamHost":"10.0.0.8","upstreamPort":8443,"explanation":"HTTPS"}`+
		"]}\n```")
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "生成 HTTPS 草稿")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the HTTPS draft", func() bool { return tt.HasText("git.example.com:443") })
	if !tt.HasText("（默认）") || tt.HasText("监听 :待定") {
		t.Errorf("default HTTPS address is misleading: %v", tt.Texts())
	}
	if tt.HasText("复制地址-加密站") {
		t.Errorf("a draft card must not gain the saved row's copy action: %v", tt.Texts())
	}
}

// TestAIChatExplicitEmptyDraftsShowsFeedback ensures machine-only empty JSON
// becomes an actionable note and a human bubble, never a raw JSON bubble.
func TestAIChatExplicitEmptyDraftsShowsFeedback(t *testing.T) {
	for _, reply := range []string{`{"drafts":[]}`, "```json\n{\"drafts\":[]}\n```"} {
		a := testApp(t)
		srv := fakeOpenAI(t, reply)
		a.SetAI(srv.URL, "chat-model")
		tt := tester(t, a, 1200, 760)
		if err := tt.Click("AI 对话"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		fillField(t, tt, "消息", "生成草稿")
		if err := tt.Click("发送"); err != nil {
			t.Fatal(err)
		}
		waitFor(t, tt, "the empty draft feedback", func() bool { return tt.HasText("没有可用草稿") })
		if !tt.HasText("继续描述") || !tt.HasText("（这条回复只有配置草稿）") {
			t.Errorf("empty drafts should have human feedback: %v", tt.Texts())
		}
		for _, text := range tt.Texts() {
			if strings.Contains(text, `"drafts"`) {
				t.Errorf("raw machine JSON remains in the bubble: %q", text)
			}
		}
		a.Close()
	}
}

// TestAIChatHistoricalDraftBubbleDoesNotUseLatestPreview covers two rounds:
// an invalid first reply must not start pointing at the second reply's cards.
func TestAIChatHistoricalDraftBubbleDoesNotUseLatestPreview(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, "```json\n{\"drafts\":["+
		`{"name":"缺上游","protocol":"tcp","listenPort":19140,"upstreamHost":"","upstreamPort":80,"explanation":"没有地址"}`+
		"]}\n```")
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "第一轮")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the first reply", func() bool { return tt.HasText("已丢弃 1 条不完整的草稿") })

	// The first request is finished, so replacing the handler determines only
	// the second round's response.
	srv.Config.Handler = fakeOpenAIHandler(chatReply())
	fillField(t, tt, "消息", "第二轮")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the second reply's preview", func() bool { return tt.HasText("规则草稿（2 条）") })
	tt.Frame()
	if tt.HasText("见下方预览") {
		t.Errorf("a historical bubble must not borrow the latest preview: %v", tt.Texts())
	}
	turns, _, _, _, _, _, _ := a.AIChatState()
	if len(turns) != 4 {
		t.Fatalf("both rounds should remain in history: %+v", turns)
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
			return tt.HasText("规则草稿（2 条）") && tt.HasText("导入到当前场景")
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

// TestAIChatComposerHoldsUnderFiveCards pins the page's tallest pile: the
// cap's own output — five cards and the note that says the cut — with the
// composer still whole inside the window at both sizes. The geometry this
// guards: the page once claimed its parent column's full height (Fill)
// under a window bar that also takes a share, which pushed the field and
// the send button below the fold at exactly this height while two drafts
// still fit. The note rides the preview's tail, the end the list follows,
// so the fold lands on it rather than a screen above.
func TestAIChatComposerHoldsUnderFiveCards(t *testing.T) {
	for _, sz := range [][2]int{{1280, 800}, {960, 640}} {
		w, h := sz[0], sz[1]
		a := testApp(t)
		srv := fakeOpenAI(t, chatSixReply())
		a.SetAI(srv.URL, "chat-model")
		tt := tester(t, a, w, h)

		if err := tt.Click("AI 对话"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		fillField(t, tt, "消息", "把这六个服务都转出去")
		if err := tt.Click("发送"); err != nil {
			t.Fatal(err)
		}
		waitFor(t, tt, "the capped preview", func() bool {
			return tt.HasText("规则草稿（5 条）") && tt.HasText("仅保留前 5 条")
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
			t.Errorf("%dx%d: the send button is pushed off screen: bottom %.1f > %.1f", w, h, bottom, float32(h)-pad)
		}
		// The cap's note rides the tail the list follows: inside the
		// window, not a line the fold has scrolled away.
		note, ok := tt.Find("草稿仅保留前 5 条")
		if !ok || note.Y < 0 || note.Y+note.H > float32(h) {
			t.Errorf("%dx%d: the cap's note must be in view at the tail: %+v ok=%v", w, h, note, ok)
		}
		// The list scrolls inside its own panel: the import action, the
		// very last thing the reply produced, is on screen too.
		imp, ok := tt.Find("导入到当前场景")
		if !ok || imp.Y+imp.H > float32(h) {
			t.Errorf("%dx%d: the import bar must stay in view at the end: %+v ok=%v", w, h, imp, ok)
		}
		a.Close()
	}
}
