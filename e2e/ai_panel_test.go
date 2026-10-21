package e2e

import (
	"testing"
)

// TestAIPanelWalkthrough drives the AI panel as the user does: configure
// the endpoint in the settings card, ask with one sentence, and adopt the
// draft into the form.
//
// It clicks the real 生成 button, so it also exercises the panel's
// goroutine against the race detector.
func TestAIPanelWalkthrough(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, `{"name":"面板里的 MySQL","protocol":"tcp","listenPort":9001,
		"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"明白"}`)
	tt := tester(t, a, 1200, 760)

	// Configure the endpoint through the settings card.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "接口地址", srv.URL)
	fillField(t, tt, "模型", "test-model")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.AIReady() {
		t.Fatal("the AI should be ready after the card is saved")
	}

	// Back on the mappings page, the entry appears and opens the panel.
	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("用一句话新建映射") {
		t.Fatalf("the AI panel did not open: %v", tt.Texts())
	}

	fillField(t, tt, "描述", "把内网 mysql 暴露到 9001")
	if err := tt.Click("生成"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the draft card", func() bool {
		return tt.HasText("草稿") && tt.HasText("面板里的 MySQL")
	})

	// Adopting the draft fills the ordinary form, and saving goes through
	// the same validation as a typed mapping.
	if err := tt.Click("采用草稿"); err != nil {
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
		list, _ := a.Store.ListMappings()
		return len(list) == 1
	})
	list, _ := a.Store.ListMappings()
	if list[0].Name != "面板里的 MySQL" || list[0].ListenPort != 9001 {
		t.Errorf("stored wrongly: %+v", list[0])
	}
}

// TestAIPanelShowsAnError checks that a model that answers rubbish leaves
// the panel open with the error visible, and nothing saved.
func TestAIPanelShowsAnError(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, "不是 JSON 的话")
	a.SetAI(srv.URL, "test-model")
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("✨ AI 建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "描述", "x")
	if err := tt.Click("生成"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the error text", func() bool { return tt.HasText("JSON") })
	// The adopt button only renders when a draft came back.
	if tt.HasText("采用草稿") {
		t.Errorf("no draft card should show: %v", tt.Texts())
	}
	if list, _ := a.Store.ListMappings(); len(list) != 0 {
		t.Errorf("nothing should have been saved: %+v", list)
	}
}
