package appt

import (
	"strings"
	"testing"
)

// enabledOf reads one rule's switch from the store.
func enabledOf(t *testing.T, a *App, id string) bool {
	t.Helper()
	m, err := a.Store.GetMapping(id)
	if err != nil {
		t.Fatalf("reading the rule: %v", err)
	}
	return m.Enabled
}

// namesLeft lists the rules the store still holds, in its order.
func namesLeft(t *testing.T, a *App) []string {
	t.Helper()
	list, err := a.Store.ListMappings()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list))
	for _, m := range list {
		names = append(names, m.Name)
	}
	return names
}

// TestBatchDeleteMappings removes several rules in one pass and leaves
// the counts the window shows.
func TestBatchDeleteMappings(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙")

	if err := a.DeleteMappings([]string{ids[0], ids[2]}); err != nil {
		t.Fatal(err)
	}
	if got := namesLeft(t, a); len(got) != 1 || got[0] != "乙" {
		t.Fatalf("store = %v, want [乙]", got)
	}
	if !a.Dirty() {
		t.Error("a batch delete should mark the configuration dirty")
	}
	if st, ok := a.Status(); !ok || st.Text != "已删除 2 条规则" {
		t.Errorf("status = %q %v, want 已删除 2 条规则", st.Text, ok)
	}
}

// TestBatchSetMappingsEnabled turns several rules off and back on.
func TestBatchSetMappingsEnabled(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙")

	if err := a.SetMappingsEnabled(ids, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if enabledOf(t, a, id) {
			t.Error("the rule should be off")
		}
	}
	if st, ok := a.Status(); !ok || st.Text != "已停用 3 条规则" {
		t.Errorf("status = %q %v, want 已停用 3 条规则", st.Text, ok)
	}
	if !a.Dirty() {
		t.Error("a batch flip should mark the configuration dirty")
	}

	if err := a.SetMappingsEnabled(ids, true); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !enabledOf(t, a, id) {
			t.Error("the rule should be back on")
		}
	}
	if st, ok := a.Status(); !ok || st.Text != "已启用 3 条规则" {
		t.Errorf("status = %q %v, want 已启用 3 条规则", st.Text, ok)
	}
}

// TestBatchDeleteMappingsError stops at the first rule that fails, says
// how many went before it, and keeps what the batch never reached.
func TestBatchDeleteMappingsError(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙")

	err := a.DeleteMappings([]string{ids[0], "不存在的", ids[1]})
	if err == nil {
		t.Fatal("a batch with a missing rule should fail")
	}
	if !strings.Contains(err.Error(), "已处理 1 条，第 2 条失败") {
		t.Errorf("error = %v, want the partial count in it", err)
	}
	if got := namesLeft(t, a); len(got) != 2 || got[0] != "乙" || got[1] != "丙" {
		t.Fatalf("store = %v, want [乙 丙]", got)
	}
	if !a.Dirty() {
		t.Error("the rules deleted before the failure should mark the configuration dirty")
	}
}

// TestBatchSetMappingsEnabledError stops at the first rule that fails,
// leaving the flips before it in place and the rest untouched.
func TestBatchSetMappingsEnabledError(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙")

	err := a.SetMappingsEnabled([]string{ids[0], "不存在的", ids[1]}, false)
	if err == nil {
		t.Fatal("a batch with a missing rule should fail")
	}
	if !strings.Contains(err.Error(), "已处理 1 条，第 2 条失败") {
		t.Errorf("error = %v, want the partial count in it", err)
	}
	if enabledOf(t, a, ids[0]) {
		t.Error("the rule flipped before the failure should stay off")
	}
	if !enabledOf(t, a, ids[1]) || !enabledOf(t, a, ids[2]) {
		t.Error("the rules after the failure should be untouched")
	}
}
