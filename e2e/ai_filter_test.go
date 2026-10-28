package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestAIFilterNarrowsTable walks the natural language filter end to end:
// the sentence goes to the model, the answer narrows the table to the
// matched row, the batch bar counts and acts on what survived, and
// clearing the filter puts every row back. Nothing about the rule set
// moves but the batch enable the user confirmed on the filtered list.
func TestAIFilterNarrowsTable(t *testing.T) {
	a := testApp(t)
	var ids []string
	for i, name := range []string{"筛选库甲", "筛选库乙", "筛选库丙"} {
		m, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
			Name: name, Protocol: model.TCP, ListenPort: 19501 + i,
			UpstreamHost: "127.0.0.1", UpstreamPort: 3306, Enabled: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	// The model is told to match only the second mapping.
	srv := fakeOpenAI(t, fmt.Sprintf(`{"ids":[%q],"rationale":"只有「筛选库乙」监听 19502"}`, ids[1]))
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 760)
	tt.Frame()
	fillField(t, tt, "AI 筛选", "19502")
	if err := tt.Click("开始筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the filter to land", func() bool {
		return tt.HasText("只有「筛选库乙」监听 19502")
	})

	// The table keeps the matched row alone; the others leave the frame.
	if !tt.HasText("筛选库乙") {
		t.Fatalf("the matched row should stay: %v", tt.Texts())
	}
	for _, gone := range []string{"筛选库甲", "筛选库丙"} {
		if tt.HasText(gone) {
			t.Errorf("%s should be filtered out: %v", gone, tt.Texts())
		}
	}
	// The batch bar counts what survived, not the whole table.
	if !tt.HasText("筛选结果 1 条") {
		t.Errorf("the batch bar should count the filtered list: %v", tt.Texts())
	}

	// A batch action acts on the filtered list: enabling "all" enables
	// the one row on screen and leaves the others untouched.
	if err := tt.Click("全部启用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("确认启用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the one row enabled", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		for _, m := range list {
			if m.ID == ids[1] && !m.Enabled {
				return false
			}
		}
		return true
	})
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	for _, m := range list {
		if want := m.ID == ids[1]; m.Enabled != want {
			t.Errorf("batch enable must act on the filtered list: %s enabled=%v", m.Name, m.Enabled)
		}
	}

	// Clearing the filter puts every row back on screen.
	if err := tt.Click("清除筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "every row back", func() bool {
		return tt.HasText("筛选库甲") && tt.HasText("筛选库丙")
	})
}

// TestAIFilterEmptyAnswer checks the answer "nothing matches": the table
// shows the no-match card, the rationale line says what the model meant,
// and the card's clear button retires the filter and brings every row
// back.
func TestAIFilterEmptyAnswer(t *testing.T) {
	a := testApp(t)
	for i, name := range []string{"空答甲", "空答乙"} {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
			Name: name, Protocol: model.TCP, ListenPort: 19601 + i,
			UpstreamHost: "127.0.0.1", UpstreamPort: 3306, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	srv := fakeOpenAI(t, `{"ids":[],"rationale":"没有监听 9999 的映射"}`)
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 760)
	tt.Frame()
	fillField(t, tt, "AI 筛选", "9999")
	if err := tt.Click("开始筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the empty answer to land", func() bool {
		return tt.HasText("没有监听 9999 的映射")
	})
	if !tt.HasText("没有匹配的规则") {
		t.Fatalf("an empty filter should show the no-match card: %v", tt.Texts())
	}

	// The card's clear button retires the AI filter too, not only the
	// search the card was built for.
	if err := tt.Click("清除筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "every row back", func() bool {
		return tt.HasText("空答甲") && tt.HasText("空答乙")
	})
	if _, _, query, _, ids := a.AIFilterState(); query != "" || len(ids) != 0 {
		t.Errorf("clearing should retire the filter: %q %v", query, ids)
	}
}

// TestAIFilterServerErrorClears walks the failure path: a gateway that
// refuses the filter leaves the error in red with the button that retires
// it. The failure hides no rows — that is the empty answer's business, not
// an error's — and the button puts the filter state back the page starts
// in, the way ClearAIFilter promises for a landed error.
func TestAIFilterServerErrorClears(t *testing.T) {
	a := testApp(t)
	for i, name := range []string{"报错甲", "报错乙"} {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
			Name: name, Protocol: model.TCP, ListenPort: 19701 + i,
			UpstreamHost: "127.0.0.1", UpstreamPort: 3306, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "filter down"},
		})
	}))
	t.Cleanup(srv.Close)
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 760)
	tt.Frame()
	fillField(t, tt, "AI 筛选", "3306")
	if err := tt.Click("开始筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the failure to land", func() bool {
		return tt.HasText("AI 筛选失败")
	})
	if !tt.HasText("500") {
		t.Errorf("the failure should name the status: %v", tt.Texts())
	}

	// A failed ask must not hide rows: the table keeps every mapping the
	// way a landed filter never would.
	for _, name := range []string{"报错甲", "报错乙"} {
		if !tt.HasText(name) {
			t.Errorf("%s should stay on screen after a failure: %v", name, tt.Texts())
		}
	}

	// The error's clear button retires the failure and the state behind it.
	if err := tt.Click("清除筛选"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the failure to clear", func() bool {
		return !tt.HasText("AI 筛选失败")
	})
	if _, errText, query, _, ids := a.AIFilterState(); errText != "" || query != "" || len(ids) != 0 {
		t.Errorf("clearing should retire the error: %q %q %v", errText, query, ids)
	}
}
