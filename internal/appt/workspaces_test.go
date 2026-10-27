package appt

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
	"github.com/lianxin255/jumpboard/internal/store"
)

// workspaceApp builds an app on a throwaway data directory, like the
// other appt tests.
func workspaceApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("JUMPBOARD_DATA_ROOT", t.TempDir())
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// wsMapping is one rule for a workspace, with a free listen port.
func wsMapping(name string, listen int, upstreamHost string, upstreamPort int) model.Mapping {
	return model.Mapping{
		Name: name, Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: upstreamHost, UpstreamPort: upstreamPort,
	}
}

// addRule puts a rule into a given scene, so a test can build two scenes
// without switching.
func addRule(t *testing.T, a *App, ws string, m model.Mapping) {
	t.Helper()
	if _, err := a.Store.CreateMapping(ws, m); err != nil {
		t.Fatal(err)
	}
}

// TestWorkspaceSwitchMovesTheRules pins the one thing the feature is
// for: a switch reads a different scene's rules, and the app that runs
// them is the one the scene shows.
func TestWorkspaceSwitchMovesTheRules(t *testing.T) {
	a := workspaceApp(t)
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	addRule(t, a, store.DefaultWorkspaceID, wsMapping("office", 9201, "10.0.0.1", 3306))
	addRule(t, a, home.ID, wsMapping("home", 9202, "10.0.0.2", 3306))
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	if err := a.switchWorkspace(home.ID); err != nil {
		t.Fatalf("switching: %v", err)
	}
	if got := a.ActiveWorkspace(); got != home.ID {
		t.Fatalf("active = %q, want %q", got, home.ID)
	}
	list, _ := a.WorkspacesState()
	if len(list) != 2 {
		t.Fatalf("workspaces = %+v", list)
	}
	mappings, _, _, _, _, _, _, _ := a.Snapshot()
	if len(mappings) != 1 || mappings[0].Name != "home" {
		t.Fatalf("the window should show 家里's rule: %+v", mappings)
	}
	// The store agrees: the switch is durable, not just on screen.
	if got := a.Store.ActiveWorkspace(); got != home.ID {
		t.Errorf("store active = %q", got)
	}
	// And the office rules stayed home in the other scene.
	other, err := a.Store.ListMappings(store.DefaultWorkspaceID)
	if err != nil || len(other) != 1 || other[0].Name != "office" {
		t.Fatalf("默认 should still hold 公司's rule: %+v %v", other, err)
	}
}

// TestWorkspaceSwitchRollsBackOnFailedApply pins the transactional side:
// when the target scene's rules cannot run, the engine keeps the previous
// scene's rules and the app goes back to the scene that matches them, so
// the window never lies about what is listening.
func TestWorkspaceSwitchRollsBackOnFailedApply(t *testing.T) {
	a := workspaceApp(t)
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	addRule(t, a, store.DefaultWorkspaceID, wsMapping("office", 9203, "10.0.0.1", 3306))
	// The target scene's rule needs a certificate that cannot be read. The
	// engine refuses the whole set rather than dropping the rule, which is
	// the failure the rollback exists for.
	bad := wsMapping("home", 9204, "10.0.0.1", 443)
	bad.Protocol = model.HTTPS
	bad.ServerNames = "home.example.com"
	bad.ListenPath = "/"
	bad.TLSCertPath = "/nonexistent/cert.pem"
	bad.TLSKeyPath = "/nonexistent/key.pem"
	addRule(t, a, home.ID, bad)
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	swErr := a.switchWorkspace(home.ID)
	if swErr == nil {
		t.Fatal("the switch should have failed")
	}
	if got := a.ActiveWorkspace(); got != store.DefaultWorkspaceID {
		t.Errorf("active = %q, want to be back on %q", got, store.DefaultWorkspaceID)
	}
	mappings, _, _, _, _, _, _, _ := a.Snapshot()
	if len(mappings) != 1 || mappings[0].Name != "office" {
		t.Errorf("the window should still show the office rule: %+v", mappings)
	}
	if !strings.Contains(swErr.Error(), "已回到") {
		t.Errorf("the error should say it went back: %v", swErr)
	}
}

// TestWorkspaceSwitchAppliesTheTargetScene pins that a switch applies
// the scene it lands on. That is the feature: the moment the table shows
// 家里's rules, the engine runs them. So no amber bar survives a switch —
// the switch is the apply.
func TestWorkspaceSwitchAppliesTheTargetScene(t *testing.T) {
	a := workspaceApp(t)
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	addRule(t, a, store.DefaultWorkspaceID, wsMapping("office", 9204, "10.0.0.1", 3306))
	addRule(t, a, home.ID, wsMapping("home", 9205, "10.0.0.2", 3306))
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	// An edit that is not applied: the amber bar goes up for 默认.
	rules, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules: %+v %v", rules, err)
	}
	if err := a.SetMappingEnabled(rules[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if !a.Dirty() {
		t.Fatal("an unapplied toggle should have marked the scene dirty")
	}
	// Switching applies the target scene, so whatever the app lands on is
	// running — including the edit the source scene left unapplied.
	if err := a.switchWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	if a.Dirty() {
		t.Error("a switch applies the target scene, so it should not be dirty")
	}
	// The source scene's edit is still in its database — switching did not
	// discard it — so switching back runs it and shows it again, with the
	// toggle applied.
	if err := a.switchWorkspace(store.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if a.Dirty() {
		t.Error("the switch applied the source scene too")
	}
	rules, err = a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(rules) != 1 || rules[0].Enabled {
		t.Fatalf("the unapplied toggle should have been applied on return: %+v %v", rules, err)
	}
}

// TestWorkspaceCRUDGuards pins the refusals: no empty name, no deleting
// the last scene, no deleting the active one, and a rename landing where
// the UI reads it.
func TestWorkspaceCRUDGuards(t *testing.T) {
	a := workspaceApp(t)
	if _, err := a.Store.CreateWorkspace("   "); err == nil {
		t.Error("a blank name should be refused")
	}
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	// The active scene — 默认, where the app started — is refused.
	if err := a.DeleteWorkspaceAction(store.DefaultWorkspaceID); !errors.Is(err, store.ErrWorkspaceInUse) {
		t.Errorf("deleting the active scene: %v", err)
	}
	// A rename lands where the UI reads it.
	if err := a.RenameWorkspaceAction(home.ID, "家庭"); err != nil {
		t.Fatal(err)
	}
	if got := a.WorkspaceName(home.ID); got != "家庭" {
		t.Errorf("renamed name = %q, want 家庭", got)
	}
	// Switching to 家庭 makes it the one the app runs, so it is the one
	// refused now, and 默认 — not active anymore — deletes.
	if err := a.switchWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteWorkspaceAction(home.ID); !errors.Is(err, store.ErrWorkspaceInUse) {
		t.Errorf("deleting the active scene after a switch: %v", err)
	}
	if err := a.DeleteWorkspaceAction(store.DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	// One scene left, and it is active. The appt layer names the active
	// scene first — that is the reason a user will understand — while the
	// store's own invariant still keeps one scene, so either refusal is
	// honest; the important fact is that nothing deletes.
	if err := a.DeleteWorkspaceAction(home.ID); !errors.Is(err, store.ErrWorkspaceInUse) && !errors.Is(err, store.ErrLastWorkspace) {
		t.Errorf("deleting the last active scene: %v", err)
	}
}

// TestImportAsNewWorkspaceLeavesTheActiveScene pins the safest import:
// a backup lands in a scene of its own, and the running rules are
// untouched.
func TestImportAsNewWorkspaceLeavesTheActiveScene(t *testing.T) {
	a := workspaceApp(t)
	addRule(t, a, store.DefaultWorkspaceID, wsMapping("office", 9206, "10.0.0.1", 3306))
	backup, err := json.Marshal(services.Payload{
		Version:    2,
		ExportedAt: 1,
		Mappings: []model.Mapping{{
			Name: "imported", Protocol: model.TCP, Enabled: true, ListenPort: 9207,
			UpstreamHost: "10.0.0.9", UpstreamPort: 3306,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	name, err := a.ImportAsNewWorkspace(backup, "同事的备份")
	if err != nil {
		t.Fatal(err)
	}
	if name != "同事的备份" {
		t.Errorf("name = %q", name)
	}
	// The active scene is untouched.
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 || list[0].Name != "office" {
		t.Fatalf("the active scene should be untouched: %+v %v", list, err)
	}
	// A scene with the imported rules exists.
	wl, err := a.Store.ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range wl {
		if w.Name == "同事的备份" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("the imported scene should exist")
	}
}
