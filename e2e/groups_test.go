package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestGroupChipFilterAndEdit walks the group feature as a user meets it:
// the table says each rule's group as a chip, the chips under the search
// box narrow the table to one group and back, and the form keeps the
// group through an edit saved without touching it.
func TestGroupChipFilterAndEdit(t *testing.T) {
	a := testApp(t)
	for _, m := range []model.Mapping{
		{Name: "甲组站", Group: "数据库", Protocol: model.TCP, Enabled: true,
			ListenPort: freePort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{Name: "乙组站", Group: "内网", Protocol: model.TCP, Enabled: true,
			ListenPort: freePort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 1},
	} {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
	}
	// A direct store write does not refresh the app's snapshot; the page
	// sees the rows after a reload, the way it does after startup.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "both rules in the table", func() bool {
		return tt.HasText("甲组站") && tt.HasText("乙组站")
	})

	// Each row carries its group as a chip, named for the test to find.
	if !tt.HasText("分组 数据库") || !tt.HasText("分组 内网") {
		t.Fatalf("the group chips are missing: %v", tt.Texts())
	}

	// Picking a group leaves the other group's rule out of the table.
	if err := tt.Click("组 数据库"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("甲组站") {
		t.Fatalf("the selected group's rule vanished: %v", tt.Texts())
	}
	if tt.HasText("乙组站") {
		t.Errorf("the other group's rule should be filtered out: %v", tt.Texts())
	}

	// 全部 brings both back.
	if err := tt.Click("组 全部"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both rules back", func() bool {
		return tt.HasText("甲组站") && tt.HasText("乙组站")
	})

	// Edit the first rule without touching anything: the form shows its
	// group, and saving keeps it in the store.
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f, _ := a.FormState()
	if f.Group != "数据库" {
		t.Fatalf("the form lost the group: %q", f.Group)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the edit saved", func() bool {
		list, err := a.Store.ListMappings(a.ActiveWorkspace())
		if err != nil || len(list) != 2 {
			return false
		}
		for _, m := range list {
			if m.Name == "甲组站" && m.Group != "数据库" {
				return false
			}
		}
		return true
	})
}
