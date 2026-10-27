package e2e

import (
	"testing"
)

// TestAIMultiDraftFlow walks the multi-draft panel the way a person does:
// one sentence naming two services becomes two cards; adopting the first
// opens the form and stores exactly one mapping; the second card keeps
// waiting until it is dropped, and dropping it changes nothing.
func TestAIMultiDraftFlow(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, `{"drafts":[
		{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL 到 9000"},
		{"name":"内网 Redis","protocol":"tcp","listenPort":9001,"upstreamHost":"192.168.1.11","upstreamPort":6379,"explanation":"Redis 到 9001"}]}`)
	a.SetAI(srv.URL, "test-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "描述", "把内网 MySQL 和 Redis 分别暴露到 9000/9001")
	if err := tt.Click("生成"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both drafts", func() bool {
		return tt.HasText("草稿（2 条）") && tt.HasText("内网 MySQL") && tt.HasText("内网 Redis")
	})
	// Not adopting anything yet stores nothing.
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Fatalf("drafts must not store anything: %+v", list)
	}

	// Adopting the first draft opens the form prefilled, and saving goes
	// through the ordinary validation.
	if err := tt.Click("采用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	open, _, f, _ := a.FormState()
	if !open || f.Name != "内网 MySQL" || f.ListenPort != "9000" {
		t.Fatalf("the form was not filled from the first draft: open=%v %+v", open, f)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping stored", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 1
	})

	// The panel kept the second card; dropping it leaves the stored
	// count at one and the table unchanged.
	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("内网 Redis") {
		t.Fatalf("the unadopted draft should still be waiting: %v", tt.Texts())
	}
	if err := tt.Click("丢弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if len(list) != 1 || list[0].Name != "内网 MySQL" {
		t.Fatalf("only the adopted draft may be stored: %+v", list)
	}
}
