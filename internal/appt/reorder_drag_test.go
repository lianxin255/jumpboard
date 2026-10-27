package appt

import (
	"net"
	"slices"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// freeTCPPort asks the system for a port nothing holds.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// seedRules puts count named rules in the store, in the order given, and
// returns their ids in that order.
func seedRules(t *testing.T, a *App, names ...string) []string {
	t.Helper()
	ids := make([]string, 0, len(names))
	for _, name := range names {
		m := model.Mapping{
			Name: name, Protocol: model.TCP, Enabled: true,
			ListenPort: freeTCPPort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
		got, err := a.Store.ListMappings(a.ActiveWorkspace())
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			if g.Name == name {
				ids = append(ids, g.ID)
				break
			}
		}
	}
	return ids
}

// order reads the store's ordering, by name for readable failures.
func order(t *testing.T, a *App) []string {
	t.Helper()
	ids, err := a.Store.MappingIDs(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		m, err := a.Store.GetMapping(id)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, m.Name)
	}
	return names
}

// TestReorderFilteredMappingsWholeList is the plain drag: the table shows
// every rule, and the order it hands over becomes the store's order.
func TestReorderFilteredMappingsWholeList(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "一号", "二号", "三号")

	if err := a.ReorderFilteredMappings([]string{ids[2], ids[0], ids[1]}); err != nil {
		t.Fatal(err)
	}
	if got := order(t, a); got[0] != "三号" || got[1] != "一号" || got[2] != "二号" {
		t.Fatalf("order = %v", got)
	}
	if a.Dirty() {
		t.Error("a drag must not mark the configuration dirty")
	}
}

// TestReorderFilteredMappingsUnderFilter checks the narrowed case: the
// rows the drag could not see keep their slots, and the shown ones take
// the shown slots in the order the drag left them.
func TestReorderFilteredMappingsUnderFilter(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙")
	// The table showed 甲 and 丙 (乙 filtered out) and the drag put 丙
	// first among them.
	if err := a.ReorderFilteredMappings([]string{ids[2], ids[0]}); err != nil {
		t.Fatal(err)
	}
	if got := order(t, a); got[0] != "丙" || got[1] != "乙" || got[2] != "甲" {
		t.Fatalf("order = %v, want 丙 乙 甲", got)
	}
}

// TestReorderFilteredMappingsRefusesStale refuses a drag whose rows do
// not match the store, rather than silently dropping or duplicating rows.
func TestReorderFilteredMappingsRefusesStale(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "一号", "二号")
	if err := a.ReorderFilteredMappings([]string{ids[0], "不存在的"}); err == nil {
		t.Fatal("a stale list should be refused")
	}
	if got := order(t, a); got[0] != "一号" || got[1] != "二号" {
		t.Fatalf("a refused drag must not touch the order: %v", got)
	}
}

// TestReorderFilteredMappingsMovesSeveral is the multi-select drag: rows
// that are not neighbours, chosen together, land as one block, and the
// rows the filter hid keep their slots around it.
func TestReorderFilteredMappingsMovesSeveral(t *testing.T) {
	a := newTestApp(t)
	ids := seedRules(t, a, "甲", "乙", "丙", "丁", "戊")
	// The table showed 甲, 丙 and 戊 (乙 and 丁 filtered out); 甲 and 戊
	// were chosen together and dragged to below 丙.
	if err := a.ReorderFilteredMappings([]string{ids[2], ids[0], ids[4]}); err != nil {
		t.Fatal(err)
	}
	want := []string{"丙", "乙", "甲", "丁", "戊"}
	if got := order(t, a); !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if a.Dirty() {
		t.Error("a drag must not mark the configuration dirty")
	}
}
