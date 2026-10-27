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
	if err := tt.Click("全部启用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("确认启用") {
		t.Fatalf("confirm should open: %v", tt.Texts())
	}
	if err := tt.Click("确认启用"); err != nil {
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
	tt.Frame()
	if err := tt.Click("确认停用"); err != nil {
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
