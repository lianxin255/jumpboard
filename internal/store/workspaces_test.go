package store

import (
	"errors"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestWorkspacesStartWithDefault pins the starting point: a fresh
// database has exactly one scene, 默认, and it is the active one.
func TestWorkspacesStartWithDefault(t *testing.T) {
	s := open(t)
	list, err := s.ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != DefaultWorkspaceID || list[0].Name != DefaultWorkspaceName {
		t.Fatalf("a fresh store should hold one 默认 scene: %+v", list)
	}
	if got := s.ActiveWorkspace(); got != DefaultWorkspaceID {
		t.Errorf("active = %q, want %q", got, DefaultWorkspaceID)
	}
}

// TestWorkspacesScoped lists a scene's mappings and leaves another's
// alone — the rule book the whole feature hangs on.
func TestWorkspacesScoped(t *testing.T) {
	s := open(t)
	home, err := s.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMapping(DefaultWorkspaceID, model.Mapping{
		Name: "office", Protocol: model.TCP, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMapping(home.ID, model.Mapping{
		Name: "home", Protocol: model.TCP, ListenPort: 9100,
		UpstreamHost: "10.0.0.2", UpstreamPort: 3306, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListMappings(home.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "home" {
		t.Fatalf("家里 should hold only its own rule: %+v", got)
	}
	got, err = s.ListEnabledMappings(DefaultWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "office" {
		t.Fatalf("默认 should hold only its own rule: %+v", got)
	}
	// Sort orders are per scene, so a scene's first rule starts at zero
	// whatever another scene's table holds.
	if got[0].SortOrder != 0 {
		t.Errorf("sort_order = %d, want 0", got[0].SortOrder)
	}
}

// TestWorkspacesActiveTracksSetting pins that writing the active scene is
// the whole switch at the store's level, and that a stale pointer heals
// itself on the next read.
func TestWorkspacesActiveTracksSetting(t *testing.T) {
	s := open(t)
	home, err := s.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	if got := s.ActiveWorkspace(); got != home.ID {
		t.Errorf("active = %q, want %q", got, home.ID)
	}
	// A pointer to a scene that is not there is the unset case: the first
	// scene wins, and the fix is written back so it stays. The delete
	// refuses the active scene, so the stale pointer is what a store from
	// an older build — or another process — can leave behind.
	if err := s.SetSetting(SettingActiveWorkspace, "gone-scene"); err != nil {
		t.Fatal(err)
	}
	if got := s.ActiveWorkspace(); got != DefaultWorkspaceID {
		t.Errorf("a stale pointer should fall back to %q, got %q", DefaultWorkspaceID, got)
	}
	// The heal is written back, so the next read agrees.
	if got := s.Setting(SettingActiveWorkspace, ""); got != DefaultWorkspaceID {
		t.Errorf("the heal should be stored, got %q", got)
	}
}

// TestDeleteWorkspaceRefusesLastAndActive pins the two refusals: a scene
// that is the last one stays, and so does the one the app runs.
func TestDeleteWorkspaceRefusesLastAndActive(t *testing.T) {
	s := open(t)
	home, err := s.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetActiveWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	// The active scene is refused outright.
	if err := s.DeleteWorkspace(home.ID); !errors.Is(err, ErrWorkspaceInUse) {
		t.Fatalf("deleting the active scene: %v", err)
	}
	// Switch away, then the refusal is about being the last one.
	if err := s.SetActiveWorkspace(DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(DefaultWorkspaceID); !errors.Is(err, ErrLastWorkspace) {
		t.Fatalf("deleting the last scene: %v", err)
	}
	// The last scene still holds its rules.
	if list, err := s.ListMappings(DefaultWorkspaceID); err != nil || len(list) != 0 {
		t.Fatalf("the last scene should survive empty: %+v %v", list, err)
	}
}

// TestDeleteWorkspaceTakesItsMappings pins that a scene's rules go with
// it, and only its own.
func TestDeleteWorkspaceTakesItsMappings(t *testing.T) {
	s := open(t)
	home, err := s.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMapping(home.ID, model.Mapping{
		Name: "home", Protocol: model.TCP, ListenPort: 9100,
		UpstreamHost: "10.0.0.2", UpstreamPort: 3306, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMapping(DefaultWorkspaceID, model.Mapping{
		Name: "office", Protocol: model.TCP, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorkspace(home.ID); err != nil {
		t.Fatal(err)
	}
	if list, err := s.ListMappings(home.ID); err != nil || len(list) != 0 {
		t.Fatalf("家里 should be gone with its rules: %+v %v", list, err)
	}
	if list, err := s.ListMappings(DefaultWorkspaceID); err != nil || len(list) != 1 {
		t.Fatalf("默认 should keep its rules: %+v %v", list, err)
	}
}
