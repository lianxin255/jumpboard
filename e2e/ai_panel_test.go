package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// refusingGateway is an endpoint that answers 500 on everything, so a
// request fails the way a broken gateway does.
func refusingGateway(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// oneDraftReply is the fake model's chat answer for the walkthrough
// tests: words around one fenced json block, the shape the chat system
// prompt asks for, carrying a single draft.
func oneDraftReply(port int, name string) string {
	return "草稿如下：\n\n```json\n{\"name\":\"" + name + "\",\"protocol\":\"tcp\",\"listenPort\":" +
		itoa(port) + ",\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306,\"explanation\":\"明白\"}\n```\n"
}

// TestAIMappingEntryGoesToChat pins the entry's new contract: the ✨
// button on the mappings page is a door to the AI chat page, not a
// dialog of its own. The page it lands on carries the composer, and no
// dialog rides over it — the form a draft later fills in opens alone.
func TestAIMappingEntryGoesToChat(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, chatReply())
	a.SetAI(srv.URL, "test-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page() != appt.PageAIChat {
		t.Fatalf("the entry should land on the AI chat page, got %v", a.Page())
	}
	for _, want := range []string{"发送", "说说你想转发的服务"} {
		if !tt.HasText(want) {
			t.Errorf("the chat page should show %q: %v", want, tt.Texts())
		}
	}
	// The retired panel's own words must not come back either.
	if tt.HasText("用一句话新建映射") {
		t.Errorf("the one-sentence panel should be gone: %v", tt.Texts())
	}
	if open, _, _, _ := a.FormState(); open {
		t.Error("no form should open from the entry itself")
	}
}

// TestAIMappingWalkthrough drives the converged flow the way a user
// does: configure the endpoint in the settings card, take the mappings
// page's AI entry to the chat page, ask with one sentence through the
// real composer, and fill the draft into the form.
//
// It clicks the real 发送 button, so it also exercises the chat's
// goroutine against the race detector.
func TestAIMappingWalkthrough(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, oneDraftReply(9001, "面板里的 MySQL"))
	tt := tester(t, a, 1200, 900)

	// Configure the endpoint through the settings card.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The card explains what each kind of model is for, and offers the
	// optional diagnosis model beside the main one.
	if !tt.HasText("fast 类模型") {
		t.Errorf("the model advice line is missing: %v", tt.Texts())
	}
	// The AI card sits below the fold of the window's default height, so
	// the fields and the save are reached the way a user reaches them: by
	// scrolling to them.
	fillFieldScrolled(t, tt, "接口地址", srv.URL)
	fillFieldScrolled(t, tt, "模型", "test-model")
	fillFieldScrolled(t, tt, "诊断/解读模型", "diag-model")
	clickScrolled(t, tt, "保存")
	tt.Frame()
	if !a.AIReady() {
		t.Fatal("the AI should be ready after the card is saved")
	}
	if a.AIDiagModel() != "diag-model" {
		t.Fatalf("the diagnosis model should round-trip, got %q", a.AIDiagModel())
	}

	// Back on the mappings page, the entry leads to the chat page, and
	// the conversation's composer takes the sentence there.
	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page() != appt.PageAIChat {
		t.Fatalf("the entry should land on the AI chat page, got %v", a.Page())
	}

	fillField(t, tt, "消息", "把内网 mysql 暴露到 9001")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the draft card", func() bool {
		return tt.HasText("规则草稿（1 条）") && tt.HasText("面板里的 MySQL")
	})

	// Filling the draft in opens the ordinary form over the chat page —
	// the one dialog on it — and saving goes through the same validation
	// as a typed mapping.
	if err := tt.Click("填入表单 面板里的 MySQL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	open, _, f, _ := a.FormState()
	if !open || f.Name != "面板里的 MySQL" || f.UpstreamPort != "3306" {
		t.Fatalf("the form was not filled from the draft: open=%v %+v", open, f)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping in the table", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 1
	})
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if list[0].Name != "面板里的 MySQL" || list[0].ListenPort != 9001 {
		t.Errorf("stored wrongly: %+v", list[0])
	}
}

// TestAIMappingShowsAnError checks that a gateway that refuses the
// request leaves the error visible on the chat page, and nothing saved.
func TestAIMappingShowsAnError(t *testing.T) {
	a := testApp(t)
	srv := refusingGateway(t)
	a.SetAI(srv.URL, "test-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "x")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the error text", func() bool { return tt.HasText("AI 服务返回 500") })
	// The fill button only renders when a draft came back.
	if tt.HasText("填入表单") {
		t.Errorf("no draft card should show: %v", tt.Texts())
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Errorf("nothing should have been saved: %+v", list)
	}
}
