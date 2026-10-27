package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The workspace a fresh database starts with, and the workspace every
// mapping from before workspaces belongs to: the column's default.
const (
	// DefaultWorkspaceID is the id of the 默认 scene.
	DefaultWorkspaceID = "default"
	// DefaultWorkspaceName is what the default scene is called.
	DefaultWorkspaceName = "默认"
)

// ErrLastWorkspace is returned when the delete would leave none: an app
// with no workspace has nowhere to put a rule, so one always stays.
var ErrLastWorkspace = errors.New("至少保留一个场景")

// ErrWorkspaceInUse is returned when the delete names the workspace the
// app is running: deleting the scene under the running rules leaves the
// active pointer dangling, so the switch happens first or not at all.
var ErrWorkspaceInUse = errors.New("不能删除当前使用的场景，请先切换到其他场景")

// EnsureWorkspaces creates the default workspace when the database has
// none. A database whose only workspace was deleted on purpose keeps the
// set it has: unlike the local host, which every mapping's foreign key
// needs, the default workspace is only a starting point.
func (s *Store) EnsureWorkspaces() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO workspaces (id, name, created_at) VALUES (?, ?, ?)`,
		DefaultWorkspaceID, DefaultWorkspaceName, nowMillis())
	return err
}

// ListWorkspaces returns every scene, oldest first — the order the chips
// row and the settings card show them in.
func (s *Store) ListWorkspaces() ([]model.Workspace, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM workspaces ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Workspace
	for rows.Next() {
		var w model.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CountWorkspaces says how many scenes exist.
func (s *Store) CountWorkspaces() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&n)
	return n, err
}

// GetWorkspace returns one scene.
func (s *Store) GetWorkspace(id string) (model.Workspace, error) {
	var w model.Workspace
	err := s.db.QueryRow(`SELECT id, name, created_at FROM workspaces WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Workspace{}, ErrNotFound
	}
	return w, err
}

// CleanWorkspaceName trims and caps a scene's name, the same limit a
// group label obeys; "" says the name is not usable.
func CleanWorkspaceName(name string) string {
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 50 {
		return string(r[:50])
	}
	return name
}

// CreateWorkspace adds a scene.
func (s *Store) CreateWorkspace(name string) (model.Workspace, error) {
	name = CleanWorkspaceName(name)
	if name == "" {
		return model.Workspace{}, fmt.Errorf("场景名称不能为空")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	w := model.Workspace{ID: newID(), Name: name, CreatedAt: nowMillis()}
	if _, err := s.db.Exec(`INSERT INTO workspaces (id, name, created_at) VALUES (?, ?, ?)`,
		w.ID, w.Name, w.CreatedAt); err != nil {
		return model.Workspace{}, err
	}
	return w, nil
}

// RenameWorkspace renames a scene.
func (s *Store) RenameWorkspace(id, name string) error {
	name = CleanWorkspaceName(name)
	if name == "" {
		return fmt.Errorf("场景名称不能为空")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE workspaces SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteWorkspace removes a scene and its mappings together. The last
// scene is refused first: an app with no scene has nowhere to put a
// rule, so one always remains even if the user meant to delete the only
// one. Deleting the scene the app runs is refused on top of that, since
// the switch is the user's own click — and the store refuses it either
// way so a path that forgets to check cannot strand the active pointer.
func (s *Store) DeleteWorkspace(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&n); err != nil {
		return err
	}
	// The last scene is refused before the active one: a database holding
	// a single scene has it active by definition, so asking whether that
	// scene is the active one would answer the wrong question about it.
	if n <= 1 {
		return ErrLastWorkspace
	}
	if id == s.activeWorkspaceLocked() {
		return ErrWorkspaceInUse
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	// The workspace's column carries no foreign key — ALTER TABLE could
	// not have added one with a non-null default — so the mappings go
	// here, in the same transaction, and a scene never leaves rules
	// behind that no scene claims.
	if _, err := tx.Exec(`DELETE FROM mappings WHERE workspace_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ActiveWorkspace returns the id of the scene the app runs. An unset or
// stale value — a setting from a deleted scene — falls back to the first
// scene, and the fallback is written back so a restart lands the same
// place rather than healing again.
func (s *Store) ActiveWorkspace() string {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.activeWorkspaceLocked()
}

// activeWorkspaceLocked is ActiveWorkspace for a caller that already
// holds writeMu: it reads without taking the lock, and it heals a stale
// pointer by writing the answer back.
func (s *Store) activeWorkspaceLocked() string {
	var id string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, SettingActiveWorkspace).Scan(&id)
	if err == nil && id != "" {
		if ok, _ := workspaceExists(s, id); ok {
			return id
		}
	}
	id = ""
	rows, err := s.db.Query(`SELECT id FROM workspaces ORDER BY rowid LIMIT 1`)
	if err != nil {
		return DefaultWorkspaceID
	}
	if rows.Next() {
		_ = rows.Scan(&id)
	}
	rows.Close()
	if id != "" {
		_ = s.setSettingLocked(SettingActiveWorkspace, id)
		return id
	}
	// No scenes at all: the caller has not run EnsureWorkspaces. The
	// default is the honest answer — it is where a mapping's column says
	// its rules live — and it is not written back, because there is no
	// scene to be active.
	return DefaultWorkspaceID
}

// workspaceExists says whether a scene with the id is there.
func workspaceExists(s *Store, id string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM workspaces WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// SetActiveWorkspace points the app at a scene, which the caller has
// checked exists: the engine's rules are switched by whoever called this,
// and the store only remembers which scene that was.
func (s *Store) SetActiveWorkspace(id string) error {
	if ok, err := workspaceExists(s, id); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return s.SetSetting(SettingActiveWorkspace, id)
}
