package appt

import (
	"errors"
	"fmt"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/services"
	"github.com/lianxin255/jumpboard/internal/store"
)

// WorkspacesState returns the scene list and the active scene's id, copied
// so a frame can walk them without holding the lock.
func (a *App) WorkspacesState() (list []model.Workspace, active string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]model.Workspace(nil), a.workspaces...), a.activeWS
}

// ActiveWorkspace returns the id of the scene whose rules the app runs
// and shows.
func (a *App) ActiveWorkspace() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeWS
}

// ActiveWorkspaceName returns the active scene's name, for a switch's
// confirmation and the switch's toast.
func (a *App) ActiveWorkspaceName() string {
	id := a.ActiveWorkspace()
	list, _ := a.WorkspacesState()
	for _, w := range list {
		if w.ID == id {
			return w.Name
		}
	}
	return ""
}

// WorkspaceName says what a scene is called, "" when the id names none.
func (a *App) WorkspaceName(id string) string {
	list, _ := a.WorkspacesState()
	for _, w := range list {
		if w.ID == id {
			return w.Name
		}
	}
	return ""
}

// RequestSwitchWorkspace is the click on a scene, from the frame. A clean
// scene switches at once; a dirty one is confirmed first, because a
// switch applies the target's rules and the edits waiting in this scene
// stay unapplied — that is a decision, not a side effect.
func (a *App) RequestSwitchWorkspace(id string) {
	if id == a.ActiveWorkspace() {
		return
	}
	name := a.WorkspaceName(id)
	if name == "" {
		a.Fail("场景不存在，可能刚被删除")
		return
	}
	if !a.Dirty() {
		a.RunBackground(func() error { return a.switchWorkspace(id) })
		return
	}
	a.Ask(&Confirm{
		Title: "切换场景",
		Body: fmt.Sprintf("「%s」有未应用的改动，切到「%s」后这些改动会留在原地等待应用。现在切换吗？",
			a.ActiveWorkspaceName(), name),
		Label: "切换",
		Run:   func() error { return a.switchWorkspace(id) },
	})
}

// switchWorkspace moves the app to another scene: the active pointer is
// written, the mappings shown become the scene's, and the engine is asked
// to run them. The engine's Apply is transactional on its own — a port it
// cannot bind leaves the previous rules running — so a failed switch has
// one thing left to undo, the pointer, and the app lands back on the
// scene whose rules are in fact running.
func (a *App) switchWorkspace(id string) error {
	a.mu.Lock()
	prev := a.activeWS
	a.mu.Unlock()
	if id == prev {
		return nil
	}
	w, err := a.Store.GetWorkspace(id)
	if err != nil {
		return fmt.Errorf("场景不存在，可能刚被删除: %w", err)
	}
	if err := a.Store.SetActiveWorkspace(id); err != nil {
		return fmt.Errorf("切换失败: %w", err)
	}
	if err := a.Reload(); err != nil {
		_ = a.Store.SetActiveWorkspace(prev)
		return err
	}
	if err := a.Apply(); err != nil {
		// The engine kept the previous scene's rules running; the app goes
		// back to the scene that matches them, so what the window shows is
		// what is listening. The failed scene's rules are untouched in the
		// database — the reason for the refusal is on the screen, and
		// fixing it (a port to free) and switching again is the whole
		// recovery.
		_ = a.Store.SetActiveWorkspace(prev)
		if reloadErr := a.Reload(); reloadErr != nil {
			return errors.Join(fmt.Errorf("切换到「%s」失败，且恢复原场景也失败了: %w", w.Name, err), reloadErr)
		}
		return fmt.Errorf("切换到「%s」失败，已回到「%s」，正在转发的规则未受影响: %w", w.Name, a.WorkspaceName(prev), err)
	}
	// A switch applies the scene it lands on — that is the point of it —
	// so nothing is left unapplied by the switch itself.
	a.Log.Info("已切换到场景「%s」", w.Name)
	a.setStatus(fmt.Sprintf("已切换到「%s」", w.Name), false)
	return nil
}

// CreateWorkspaceAction adds a scene. The new scene starts empty and
// stays not-active: a switch is its own click, because landing on an
// empty scene stops every listener and that should never be a side
// effect of naming one.
func (a *App) CreateWorkspaceAction(name string) error {
	w, err := a.Store.CreateWorkspace(name)
	if err != nil {
		return err
	}
	if err := a.Reload(); err != nil {
		return err
	}
	a.Log.Info("新建场景「%s」", w.Name)
	a.setStatus(fmt.Sprintf("已新建场景「%s」", w.Name), false)
	return nil
}

// RenameWorkspaceAction renames a scene. The running rules are the
// scene's, not its name's, so nothing else moves.
func (a *App) RenameWorkspaceAction(id, name string) error {
	if err := a.Store.RenameWorkspace(id, name); err != nil {
		return err
	}
	if err := a.Reload(); err != nil {
		return err
	}
	a.Log.Info("场景改名为「%s」", a.WorkspaceName(id))
	return nil
}

// DeleteWorkspaceAction removes a scene and its mappings. The last scene
// and the active one are refused — the first by the store, the second
// here with the name the message deserves — and the confirmation is the
// caller's (the UI's Ask), as with a mapping's delete.
func (a *App) DeleteWorkspaceAction(id string) error {
	if id == a.ActiveWorkspace() {
		return fmt.Errorf("%w", store.ErrWorkspaceInUse)
	}
	name := a.WorkspaceName(id)
	if err := a.Store.DeleteWorkspace(id); err != nil {
		return err
	}
	if err := a.Reload(); err != nil {
		return err
	}
	a.Log.Info("删除场景「%s」", name)
	a.setStatus(fmt.Sprintf("已删除场景「%s」", name), false)
	return nil
}

// ImportAsNewWorkspace reads a backup into a scene of its own, created
// for it: the user with 公司's backup at home does not replace 家里's
// rules to get a look at them. The scene appears only when the backup
// proves readable — a failed import leaves no empty scene named after a
// file that would not load.
func (a *App) ImportAsNewWorkspace(data []byte, name string) (string, error) {
	w, err := a.Store.CreateWorkspace(name)
	if err != nil {
		return "", err
	}
	res, err := services.Import(a.Store, w.ID, data)
	if err != nil {
		// The scene was created for this import and the import refused:
		// taking the scene back with it is the honest outcome.
		if delErr := a.Store.DeleteWorkspace(w.ID); delErr != nil {
			return "", errors.Join(fmt.Errorf("导入失败，且清理新建的场景也失败了（场景「%s」保留）: %w", w.Name, err), delErr)
		}
		return "", err
	}
	if err := a.Reload(); err != nil {
		return w.Name, err
	}
	a.Log.Info("导入备份为场景「%s」：%d 条映射，跳过 %d 条", w.Name, res.MappingsImported, res.Skipped)
	a.setStatus(fmt.Sprintf("已导入 %d 条映射为场景「%s」，切换后生效", res.MappingsImported, w.Name), false)
	return w.Name, nil
}
