package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// TestWorkspaceSwitchRoundtrip drives the switcher the way a user does: the
// chips row the scenes, a click on another scene moves the app to it — the
// table shows that scene's rules and the engine runs them — and a click
// back returns to the first scene and its rules. That is the roundtrip the
// feature exists for, and it happens through the real view and the real
// engine, not through the appt API alone.
func TestWorkspaceSwitchRoundtrip(t *testing.T) {
	a := testApp(t)
	// The tester reads the view's state as its first frame draws it, so it
	// is built before the data changes below, the way every other e2e test
	// builds it.
	tt := tester(t, a, 1200, 760)
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}

	// One rule per scene, both bound to real ports so the switch really
	// starts and stops listeners.
	officePort := freePort(t)
	homePort := freePort(t)
	mk := func(ws, name string, port int) string {
		m := model.Mapping{
			Name: name, Protocol: model.TCP, Enabled: true, ListenPort: port,
			UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}
		if _, err := a.Store.CreateMapping(ws, m); err != nil {
			t.Fatal(err)
		}
		got, err := a.Store.ListMappings(ws)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			if g.Name == name {
				return g.ID
			}
		}
		t.Fatalf("%s not found in %s", name, ws)
		return ""
	}
	office := mk(store.DefaultWorkspaceID, "公司规则", officePort)
	homeRule := mk(home.ID, "家里规则", homePort)
	// The rules were written straight into the store, so the table and the
	// chips learn of them from a Reload, the way any background action
	// reports its result.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}

	// The chips row every scene, and the table shows the active one's rule.
	waitFor(t, tt, "the 场景 chips", func() bool {
		return tt.HasText("场景 家里") && tt.HasText("场景 默认")
	})
	if !tt.HasText("公司规则") {
		t.Fatalf("the default scene should be shown: %v", tt.Texts())
	}

	// Switch to 家里 through the chip.
	if err := tt.Click("场景 家里"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "家里's rule in the table", func() bool { return tt.HasText("家里规则") })
	if tt.HasText("公司规则") {
		t.Error("公司's rule should be out of the table after the switch")
	}
	if a.ActiveWorkspace() != home.ID {
		t.Errorf("active workspace = %q, want %q", a.ActiveWorkspace(), home.ID)
	}
	if a.Store.ActiveWorkspace() != home.ID {
		t.Error("the switch should be durable in the settings")
	}
	// The engine runs the scene it landed on.
	if listening, problem := a.RuleState(homeRule); !listening {
		t.Errorf("家里's rule should be listening after the switch: %q", problem)
	}
	if listening, _ := a.RuleState(office); listening {
		t.Error("公司's rule should have stopped when the scene left it")
	}

	// And back.
	if err := tt.Click("场景 默认"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "公司's rule back in the table", func() bool { return tt.HasText("公司规则") })
	if tt.HasText("家里规则") {
		t.Error("家里's rule should be out of the table after switching back")
	}
	if a.ActiveWorkspace() != store.DefaultWorkspaceID {
		t.Errorf("active workspace = %q", a.ActiveWorkspace())
	}
	if listening, problem := a.RuleState(office); !listening {
		t.Errorf("公司's rule should be listening after switching back: %q", problem)
	}
}
