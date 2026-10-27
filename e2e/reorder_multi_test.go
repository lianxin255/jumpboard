package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestMultiSelectDragReorders drives the table's choice: rows chosen
// together — a click, a ⇧-click's run, a ⌘-click in and out — drag as one
// block through the same store call a single row's drag takes, a filter
// between the choice and the store hides rows the drag cannot move, and a
// row grabbed outside the choice still travels alone.
func TestMultiSelectDragReorders(t *testing.T) {
	a := testApp(t)
	mk := func(name string) {
		m := streamRule(t, name)
		if _, err := a.Store.CreateMapping(m); err != nil {
			t.Fatal(err)
		}
	}
	// Three rules carry 筛, so the search box can narrow the table to
	// exactly them and leave the other two out of what a drag can see.
	mk("筛甲")
	mk("乙")
	mk("筛丙")
	mk("丁")
	mk("筛戊")
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	nameByID := func() []string {
		ids, err := a.Store.MappingIDs()
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

	tt := tester(t, a, 1200, 760)
	after := func(want ...string) bool {
		texts := tt.Texts()
		at := -1
		for _, w := range want {
			found := false
			for i := at + 1; i < len(texts); i++ {
				if texts[i] == w {
					at = i
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	inOrder := func(want ...string) bool {
		got := nameByID()
		if len(got) != len(want) {
			return false
		}
		for i, w := range want {
			if got[i] != w {
				return false
			}
		}
		return true
	}

	// A row is chosen on its band in the 协议 column, where nothing in
	// the row answers a click of its own: not the name, which opens the
	// detail popover, and not the address, which copies.
	proto, ok := tt.Find("协议")
	if !ok {
		t.Fatalf("the protocol column is missing: %v", tt.Texts())
	}
	chooseX := proto.X + proto.W/2
	band := func(name string) float32 {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the row %s is missing: %v", name, tt.Texts())
		}
		return r.Y + r.H/2
	}
	choose := func(mods ui.Modifiers, name string) {
		t.Helper()
		tt.ClickAtWith(mods, chooseX, band(name))
		tt.Frame()
	}

	// A click chooses 筛甲, and ⇧ extends the choice through 筛丙 to it,
	// 乙 between them included — a run, not a pair.
	choose(0, "筛甲")
	choose(ui.Shift, "筛丙")
	// The block is dragged by any row it holds, pressed the way a pointer
	// drag goes, and dropped under 丁's middle — before 筛戊 — landing
	// the three rows there together.
	from, _ := tt.Find("筛甲")
	under, _ := tt.Find("丁")
	dragRow(t, tt, from.X+from.W/2, from.Y+from.H/2, under.X+under.W/2, under.Y+under.H*0.6)
	waitFor(t, tt, "the block to land", func() bool {
		return after("丁", "筛甲", "乙", "筛丙", "筛戊") && inOrder("丁", "筛甲", "乙", "筛丙", "筛戊")
	})

	// The search narrows the table to the 筛 rows, so 乙 and 丁 leave the
	// list — and the drag below cannot touch them. The choice is made
	// again on what is shown: a click, then a ⌘-click that adds a row.
	fillField(t, tt, "搜索规则", "筛")
	choose(0, "筛甲")
	choose(ui.Cmd, "筛丙")
	from, _ = tt.Find("筛甲")
	last, _ := tt.Find("筛戊")
	dragRow(t, tt, from.X+from.W/2, from.Y+from.H/2, last.X+last.W/2, last.Y+last.H*0.9)
	waitFor(t, tt, "the choice to reach the end of the narrowed table", func() bool {
		return after("筛戊", "筛甲", "筛丙") && inOrder("丁", "筛戊", "乙", "筛甲", "筛丙")
	})

	// Back on the whole list, the choice is still held — and a row
	// grabbed outside it travels alone: 乙 moves, the choice does not.
	if err := tt.Click("清空搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	from, _ = tt.Find("乙")
	last, _ = tt.Find("筛丙")
	dragRow(t, tt, from.X+from.W/2, from.Y+from.H/2, last.X+last.W/2, last.Y+last.H*0.9)
	waitFor(t, tt, "the unchosen row to reach the end", func() bool {
		return inOrder("丁", "筛戊", "筛甲", "筛丙", "乙")
	})
	if a.Dirty() {
		t.Error("a drag must not mark the configuration dirty")
	}
}
