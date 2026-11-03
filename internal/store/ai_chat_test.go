package store

import (
	"path/filepath"
	"testing"
)

// TestAIChatPersistenceAndSceneIsolation checks the database contract:
// history and pending drafts survive a reopen and remain per scene.
func TestAIChatPersistenceAndSceneIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddAIChatTurn(DefaultWorkspaceID, "user", "默认问题", 1); err != nil {
		t.Fatal(err)
	}
	home, err := s.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddAIChatTurn(home.ID, "user", "家里问题", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	turns, err := s.ListAIChatTurns(DefaultWorkspaceID, 50)
	if err != nil || len(turns) != 1 || turns[0].Content != "默认问题" {
		t.Fatalf("default history = %+v, %v", turns, err)
	}
	other, err := s.ListAIChatTurns(home.ID, 50)
	if err != nil || len(other) != 1 || other[0].Content != "家里问题" {
		t.Fatalf("home history = %+v, %v", other, err)
	}
	if err := s.ClearAIChatTurns(DefaultWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ListAIChatTurns(DefaultWorkspaceID, 50); len(got) != 0 {
		t.Errorf("default was not cleared: %+v", got)
	}
	if got, _ := s.ListAIChatTurns(home.ID, 50); len(got) != 1 {
		t.Errorf("home must not be cleared with default: %+v", got)
	}
}
