package e2e

import (
	"testing"
)

// TestAIMultiDraftFlow walks the converged draft flow the way a person
// does: the mappings page's entry leads to the chat page, one sentence
// naming two services becomes two rule cards; filling the first into
// the form stores exactly one mapping; the second card keeps waiting
// until it is dropped, and dropping it changes nothing.
func TestAIMultiDraftFlow(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, chatReply())
	a.SetAI(srv.URL, "test-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "消息", "把内网 MySQL 和 Redis 分别暴露到 "+itoa(chatPorts[0])+"/"+itoa(chatPorts[1]))
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both drafts", func() bool {
		return tt.HasText("规则草稿（2 条）") && tt.HasText("内网 MySQL") && tt.HasText("内网 Redis")
	})
	// The cards read as the rules they would become: the forwarding
	// relation the workbench's rows carry, not a bare sentence.
	for _, want := range []string{
		"127.0.0.1:" + itoa(chatPorts[0]), "192.168.1.10:3306", "127.0.0.1:" + itoa(chatPorts[1]), "192.168.1.11:6379",
	} {
		if !tt.HasText(want) {
			t.Errorf("the draft card should show %q: %v", want, tt.Texts())
		}
	}
	// Not importing anything yet stores nothing.
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Fatalf("drafts must not store anything: %+v", list)
	}

	// Filling the first draft into the form opens it prefilled, and
	// saving goes through the ordinary validation.
	if err := tt.Click("填入表单 内网 MySQL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	open, _, f, _ := a.FormState()
	if !open || f.Name != "内网 MySQL" || f.ListenPort != itoa(chatPorts[0]) {
		t.Fatalf("the form was not filled from the first draft: open=%v %+v", open, f)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping stored", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 1
	})

	// The page kept the second card; dropping it leaves the stored count
	// at one and the table unchanged.
	tt.Frame()
	if !tt.HasText("内网 Redis") {
		t.Fatalf("the unadopted draft should still be waiting: %v", tt.Texts())
	}
	if err := tt.Click("丢弃草稿 内网 Redis"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if len(list) != 1 || list[0].Name != "内网 MySQL" {
		t.Fatalf("only the adopted draft may be stored: %+v", list)
	}
}
