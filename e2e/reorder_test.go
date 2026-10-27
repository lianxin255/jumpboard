package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

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
