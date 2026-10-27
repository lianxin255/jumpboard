package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/model"
)

// dragRow presses a row's name, moves in steps to where it is dropped,
// and releases there — the way a pointer drag goes, so the drag machinery
// engages the same as it does by hand.
func dragRow(t *testing.T, tt *ui.Tester, fromX, fromY, toX, toY float32) {
	t.Helper()
	tt.Press(fromX, fromY)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(fromX+(toX-fromX)*f, fromY+(toY-fromY)*f)
	}
	tt.Release(toX, toY)
}

// TestMoveMappingUpAndDown checks the reorder a row's context menu drives:
// swapping one rule with its neighbour changes only the order, leaves the
// rules and the configuration untouched, and stops politely at the edges.
func TestMoveMappingUpAndDown(t *testing.T) {
	a := testApp(t)
	mk := func(name string) model.Mapping {
		m := streamRule(t, name)
		if _, err := a.Store.CreateMapping(m); err != nil {
			t.Fatal(err)
		}
		// CreateMapping writes the row but not the id back into the model
		// it was handed, so the id comes from the store.
		got, err := a.Store.ListMappings()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			if g.Name == name {
				return g
			}
		}
		t.Fatalf("created %s not found in %v", name, got)
		return model.Mapping{}
	}
	first := mk("第一条")
	second := mk("第二条")
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	before := func() []string {
		ids, err := a.Store.MappingIDs()
		if err != nil {
			t.Fatal(err)
		}
		return ids
	}

	if got := before(); len(got) != 2 || got[0] != first.ID {
		t.Fatalf("start order %v, want %s first", got, first.ID)
	}

	// Moving the first down puts the second above it.
	if err := a.MoveMapping(first.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := before(); got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("after down: %v", got)
	}
	// Moving it up restores the order.
	if err := a.MoveMapping(first.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := before(); got[0] != first.ID || got[1] != second.ID {
		t.Fatalf("after up: %v", got)
	}

	// The first cannot go up: a no-op, not an error.
	if err := a.MoveMapping(first.ID, true); err != nil {
		t.Fatalf("moving the first up should do nothing, got %v", err)
	}
	if a.Dirty() {
		t.Error("a move must not mark the configuration dirty")
	}
}

// TestDragRowReorders drives the drag the table itself offers: a row
// pressed on its name, carried over another, and released lands the order
// in the store, through the same call the context menu's 上移/下移 use —
// and a row dropped where it came from does nothing.
func TestDragRowReorders(t *testing.T) {
	a := testApp(t)
	mk := func(name string) string {
		m := streamRule(t, name)
		if _, err := a.Store.CreateMapping(m); err != nil {
			t.Fatal(err)
		}
		got, err := a.Store.ListMappings()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			if g.Name == name {
				return g.ID
			}
		}
		t.Fatalf("created %s not found", name)
		return ""
	}
	mk("拖动一号")
	mk("拖动二号")
	third := mk("拖动三号")
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

	// The store lands the reorder off the main thread; the table catches
	// up a frame later. Waiting on the table's own text (rather than the
	// store) is what makes the next drag's coordinates honest.
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

	// The drop goes below the second row's middle and above the third's —
	// before 三号 in the list as it stands — so the first lands second.
	// (Above 二号's middle would be the first row's own place: the
	// toolkit drops that as the no-op it is.)
	row1, ok := tt.Find("拖动一号")
	if !ok {
		t.Fatalf("the row is missing: %v", tt.Texts())
	}
	row2, _ := tt.Find("拖动二号")
	dragRow(t, tt, row1.X+row1.W/2, row1.Y+row1.H/2, row2.X+row2.W/2, row2.Y+row2.H*0.6)
	waitFor(t, tt, "the drag to land", func() bool {
		return after("拖动二号", "拖动一号") && nameByID()[0] == "拖动二号"
	})

	// Dropping a row where it came from changes nothing.
	row0, _ := tt.Find("拖动二号")
	dragRow(t, tt, row0.X+row0.W/2, row0.Y+row0.H/2, row0.X+row0.W/2, row0.Y+row0.H/2+6)
	tt.Frame()
	if got := nameByID(); got[0] != "拖动二号" {
		t.Fatalf("a row dropped on itself must not move: %v", got)
	}

	// A drag past the last row's middle carries the row to the end.
	row0, _ = tt.Find("拖动二号")
	last, _ := tt.Find("拖动三号")
	dragRow(t, tt, row0.X+row0.W/2, row0.Y+row0.H/2, last.X+last.W/2, last.Y+last.H*0.9)
	waitFor(t, tt, "the row to reach the end", func() bool {
		return after("拖动一号", "拖动三号", "拖动二号") && nameByID()[2] == "拖动二号"
	})
	if got := nameByID(); got[0] != "拖动一号" || got[1] != "拖动三号" || got[2] != "拖动二号" {
		t.Fatalf("final order = %v", got)
	}
	if a.Dirty() {
		t.Error("a drag must not mark the configuration dirty")
	}

	// The context menu's moves still work beside the drag.
	if err := a.MoveMapping(third, true); err != nil {
		t.Fatal(err)
	}
	// 三号 swaps with 一号: [三号, 一号, 二号].
	if got := nameByID(); got[0] != "拖动三号" || got[1] != "拖动一号" || got[2] != "拖动二号" {
		t.Fatalf("after the menu move: %v", got)
	}
}
