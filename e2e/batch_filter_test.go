package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestBatchEnableDisable turns several rules on and off from the list bar.
func TestBatchEnableDisable(t *testing.T) {
	a := testApp(t)
	for i, name := range []string{"规则甲", "规则乙", "规则丙"} {
		m := model.Mapping{
			Name: name, Protocol: model.TCP, ListenPort: 18101 + i,
			UpstreamHost: "127.0.0.1", UpstreamPort: 3306, Enabled: false,
		}
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	if !tt.HasText("全部启用") {
		t.Fatalf("batch enable should show when rules are off: %v", tt.Texts())
	}
	// Enabling is reversible, so it lands at once and the toast offers the
	// way back instead of asking first.
	if err := tt.Click("全部启用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "all enabled", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		for _, m := range list {
			if !m.Enabled {
				return false
			}
		}
		return len(list) == 3
	})

	tt.Frame()
	if !tt.HasText("全部停用") {
		t.Fatalf("batch disable should show: %v", tt.Texts())
	}
	if err := tt.Click("全部停用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "all disabled", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		for _, m := range list {
			if m.Enabled {
				return false
			}
		}
		return len(list) == 3
	})
}

// TestFilterEmptyClear clears a search that matches nothing.
func TestFilterEmptyClear(t *testing.T) {
	a := testApp(t)
	m := model.Mapping{
		Name: "仅此一条", Protocol: model.TCP, ListenPort: 19001,
		UpstreamHost: "127.0.0.1", UpstreamPort: 80, Enabled: true,
	}
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	if err := tt.Click("搜索规则"); err != nil {
		t.Fatal(err)
	}
	tt.Type("根本不存在的名字zzz")
	tt.Frame()
	if !tt.HasText("没有匹配的规则") {
		t.Fatalf("empty filter card missing: %v", tt.Texts())
	}
	if err := tt.Click("清除筛选"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("仅此一条") {
		t.Fatalf("clearing filter should show the rule again: %v", tt.Texts())
	}
}

// TestBatchEnableUndoPutsItBack covers the reason the batch bar stopped
// asking first: the change is reversible, so it lands at once and the toast
// carries the way back. If 撤销 did not actually restore the rows, dropping
// the confirmation would have traded a dialog for a dead end.
func TestBatchEnableUndoPutsItBack(t *testing.T) {
	a := testApp(t)
	for i, name := range []string{"撤销甲", "撤销乙"} {
		m := model.Mapping{
			Name: name, Protocol: model.TCP, ListenPort: 18201 + i,
			UpstreamHost: "127.0.0.1", UpstreamPort: 3306, Enabled: false,
		}
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	if err := tt.Click("全部启用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both enabled", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		for _, m := range list {
			if !m.Enabled {
				return false
			}
		}
		return len(list) == 2
	})

	tt.Frame()
	if !tt.HasText("撤销") {
		t.Fatalf("the toast should offer 撤销: %v", tt.Texts())
	}
	if err := tt.Click("撤销"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both back to disabled", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		for _, m := range list {
			if m.Enabled {
				return false
			}
		}
		return len(list) == 2
	})
}
