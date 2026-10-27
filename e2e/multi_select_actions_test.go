package e2e

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// chooseRows picks rows by name on the band the 协议 column carries — the
// one place in a row where no widget answers a click of its own: the
// first row by a plain click, each further one by a ⌘-click.
func chooseRows(t *testing.T, tt *ui.Tester, names ...string) {
	t.Helper()
	proto, ok := tt.Find("协议")
	if !ok {
		t.Fatalf("the protocol column is missing: %v", tt.Texts())
	}
	x := proto.X + proto.W/2
	for i, name := range names {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the row %s is missing: %v", name, tt.Texts())
		}
		var mods ui.Modifiers
		if i > 0 {
			mods = ui.Cmd
		}
		tt.ClickAtWith(mods, x, r.Y+r.H/2)
		tt.Frame()
	}
}

// ruleEnabled reads one rule's switch from the store.
func ruleEnabled(t *testing.T, a *appt.App, name string) bool {
	t.Helper()
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.Name == name {
			return m.Enabled
		}
	}
	t.Fatalf("the rule %s is missing from the store", name)
	return false
}

// seedNames puts the named rules in the store and reloads the app.
func seedNames(t *testing.T, a *appt.App, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
}

// TestMultiSelectBatchDisableAndEnable turns the chosen rules off and
// back on from the bar, leaving the unchosen rule alone.
func TestMultiSelectBatchDisableAndEnable(t *testing.T) {
	a := testApp(t)
	seedNames(t, a, "甲", "乙", "丙")
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	chooseRows(t, tt, "甲", "丙")
	if !tt.HasText("已选 2 条") {
		t.Fatalf("the bar should count the choice: %v", tt.Texts())
	}
	if err := tt.Click("批量停用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "甲 and 丙 off, 乙 on", func() bool {
		return !ruleEnabled(t, a, "甲") && ruleEnabled(t, a, "乙") && !ruleEnabled(t, a, "丙")
	})
	// The bar has to count the flipped choice before 批量启用 answers a
	// click: it stays disabled while the batch before it is still running.
	waitFor(t, tt, "the bar to count the flipped choice", func() bool {
		return tt.HasText("已选 2 条 · 启用 0 · 停用 2")
	})
	if err := tt.Click("批量启用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the choice back on", func() bool {
		return ruleEnabled(t, a, "甲") && ruleEnabled(t, a, "乙") && ruleEnabled(t, a, "丙")
	})
}

// TestMultiSelectBatchDelete asks before it deletes, names the count, and
// the choice goes with its rows.
func TestMultiSelectBatchDelete(t *testing.T) {
	a := testApp(t)
	seedNames(t, a, "甲", "乙", "丙")
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	chooseRows(t, tt, "甲", "乙")
	if err := tt.Click("批量删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("批量删除") || !tt.HasText("2 条规则") || !tt.HasText("确认删除") {
		t.Fatalf("the confirmation should name the count: %v", tt.Texts())
	}
	if err := tt.Click("确认删除"); err != nil {
		t.Fatal(err)
	}
	// The rows go, and the choice goes with them: once the store holds
	// only 丙 the bar falls back to the whole list.
	waitFor(t, tt, "the rows gone and the choice with them", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 1 && list[0].Name == "丙" && tt.HasText("当前列表 1 条")
	})
}

// TestMultiSelectBatchContextMenu right-clicks a chosen row and drives
// the batch actions its menu carries for the whole choice.
func TestMultiSelectBatchContextMenu(t *testing.T) {
	a := testApp(t)
	seedNames(t, a, "甲", "乙", "丙")
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	chooseRows(t, tt, "甲", "丙")
	// The row is what the name resolves to — the table labels its rows —
	// and the menu belongs to the 规则 cell, so the click goes just inside
	// the row's left edge, on the cell's own content.
	row, ok := tt.Find("甲")
	if !ok {
		t.Fatalf("the row 甲 is missing: %v", tt.Texts())
	}
	tt.RightClickAt(row.X+20, row.Y+row.H/2)
	menu := tt.Menu()
	for _, item := range []string{"批量启用", "批量停用", "批量删除…", "取消选择"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("the chosen row's menu lacks %q: %v", item, menu)
		}
	}
	if slices.Contains(menu, "编辑") {
		t.Errorf("a choice of several should not carry the single row's items: %v", menu)
	}
	if err := tt.ChooseMenuItem("批量停用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "both chosen rules off", func() bool {
		return !ruleEnabled(t, a, "甲") && ruleEnabled(t, a, "乙") && !ruleEnabled(t, a, "丙")
	})
}

// TestMultiSelectFilterIsolation checks that a batch takes only the
// chosen rows the filter still shows: a rule the search hid is left
// exactly as it was.
func TestMultiSelectFilterIsolation(t *testing.T) {
	a := testApp(t)
	seedNames(t, a, "筛甲", "乙", "筛丙")
	tt := tester(t, a, 1200, 760)
	tt.Frame()

	chooseRows(t, tt, "筛甲", "筛丙")
	fillField(t, tt, "搜索规则", "筛甲")
	tt.Frame()
	if !tt.HasText("已选 1 条") {
		t.Fatalf("the bar should count only what the filter kept: %v", tt.Texts())
	}
	if err := tt.Click("批量停用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "only the shown rule off", func() bool {
		return !ruleEnabled(t, a, "筛甲") && ruleEnabled(t, a, "乙") && ruleEnabled(t, a, "筛丙")
	})

	if err := tt.Click("清空搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !ruleEnabled(t, a, "筛丙") {
		t.Error("a rule the filter hid must not have been touched by the batch")
	}
}

// TestMultiSelectBatchMixedChoiceCount pins the toast to the rows the
// batch actually flipped: a choice holding one rule on and one off is two
// rows, but a 批量停用 flips only the one that was on, and the toast must
// say so.
func TestMultiSelectBatchMixedChoiceCount(t *testing.T) {
	a := testApp(t)
	seedNames(t, a, "甲", "乙", "丙")
	// streamRule leaves every rule on, so 乙 is flipped off: the choice of
	// 甲 (on) and 乙 (off) is mixed.
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	var offID string
	for _, m := range list {
		if m.Name == "乙" {
			offID = m.ID
		}
	}
	if offID == "" {
		t.Fatal("rule 乙 is missing from the store")
	}
	if err := a.Store.SetMappingEnabled(offID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	tt.Frame()
	chooseRows(t, tt, "甲", "乙")
	if err := tt.Click("批量停用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the one row that was on to go off", func() bool {
		return !ruleEnabled(t, a, "甲") && !ruleEnabled(t, a, "乙") && ruleEnabled(t, a, "丙")
	})
	waitFor(t, tt, "the toast to count the one flip", func() bool {
		st, ok := a.Status()
		return ok && st.Text == "已停用 1 条规则"
	})
}
