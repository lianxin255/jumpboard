package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// AIChatTurn is one locally stored conversation turn. The workspace id is
// the scene it belongs to, not the currently selected scene when it is read.
type AIChatTurn struct {
	WorkspaceID string `json:"workspaceId"`
	Role        string `json:"role"`
	Content     string `json:"content"`
	CreatedAt   int64  `json:"createdAt"`
}

// AddAIChatTurn appends a turn in the conversation's own scene. A failed
// scene switch cannot make a later reply appear under another scene.
func (s *Store) AddAIChatTurn(workspaceID, role, content string, at int64) error {
	content = strings.TrimSpace(content)
	if workspaceID == "" || content == "" || (role != ai.RoleUser && role != ai.RoleAssistant) {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	// Opening the Store directly (as storage tests and importers do) must
	// have the same valid default scene as opening the App. The INSERT is
	// a no-op once a scene exists, and keeps this write's transaction.
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO workspaces (id, name, created_at) VALUES (?, ?, ?)`, DefaultWorkspaceID, DefaultWorkspaceName, nowMillis()); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO ai_chat_turns (workspace_id, role, content, created_at) VALUES (?, ?, ?, ?)`, workspaceID, role, content, at)
	return err
}

// ListAIChatTurns returns a scene's saved history in conversation order.
func (s *Store) ListAIChatTurns(workspaceID string, limit int) ([]AIChatTurn, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT workspace_id, role, content, created_at FROM (
		SELECT rowid, workspace_id, role, content, created_at FROM ai_chat_turns
		WHERE workspace_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ?
	) ORDER BY created_at, rowid`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AIChatTurn
	for rows.Next() {
		var turn AIChatTurn
		if err := rows.Scan(&turn.WorkspaceID, &turn.Role, &turn.Content, &turn.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, turn)
	}
	return out, rows.Err()
}

// ClearAIChatTurns deletes only one scene's conversation.
func (s *Store) ClearAIChatTurns(workspaceID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM ai_chat_turns WHERE workspace_id = ?`, workspaceID)
	return err
}

// TrimAIChatTurns bounds each scene's local history independently.
func (s *Store) TrimAIChatTurns(workspaceID string, keep int) error {
	if keep <= 0 {
		keep = 50
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM ai_chat_turns WHERE workspace_id = ? AND rowid NOT IN (
		SELECT rowid FROM ai_chat_turns WHERE workspace_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ?
	)`, workspaceID, workspaceID, keep)
	return err
}

// AIChatDraft is one saved proposal and, for an edit, the version it was
// made against. The original mapping is the conflict guard, not display data.
type AIChatDraft struct {
	WorkspaceID string           `json:"workspaceId"`
	Operation   string           `json:"operation"` // "create" | "update"
	TargetID    string           `json:"targetId,omitempty"`
	Original    *json.RawMessage `json:"original,omitempty"`
	Proposed    json.RawMessage  `json:"proposed"`
	Explanation string           `json:"explanation"`
}

// SaveAIChatDrafts replaces a scene's pending proposals with the newest
// sanitized set. New mappings never get a target id or an original version.
func (s *Store) SaveAIChatDrafts(workspaceID string, drafts []AIChatDraft) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM ai_chat_drafts WHERE workspace_id = ?`, workspaceID); err != nil {
		return err
	}
	for i, draft := range drafts {
		if draft.Operation != "update" {
			draft.Operation = "create"
		}
		var original any
		if draft.Original != nil {
			original = *draft.Original
		}
		if _, err := s.db.Exec(`INSERT INTO ai_chat_drafts (workspace_id, sort_order, operation, target_id, original, proposed, explanation) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			workspaceID, i, draft.Operation, draft.TargetID, original, draft.Proposed, draft.Explanation); err != nil {
			return fmt.Errorf("saving AI draft %d: %w", i, err)
		}
	}
	return nil
}

// ListAIChatDrafts returns a scene's pending proposals in display order.
func (s *Store) ListAIChatDrafts(workspaceID string) ([]AIChatDraft, error) {
	rows, err := s.db.Query(`SELECT operation, target_id, original, proposed, explanation FROM ai_chat_drafts WHERE workspace_id = ? ORDER BY sort_order, rowid`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AIChatDraft
	for rows.Next() {
		var draft AIChatDraft
		var target, original, explanation *string
		if err := rows.Scan(&draft.Operation, &target, &original, &draft.Proposed, &explanation); err != nil {
			return nil, err
		}
		draft.WorkspaceID = workspaceID
		if target != nil {
			draft.TargetID = *target
		}
		if original != nil {
			raw := json.RawMessage(*original)
			draft.Original = &raw
		}
		if explanation != nil {
			draft.Explanation = *explanation
		}
		out = append(out, draft)
	}
	return out, rows.Err()
}

// ClearAIChatDrafts deletes only a scene's pending proposals.
func (s *Store) ClearAIChatDrafts(workspaceID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM ai_chat_drafts WHERE workspace_id = ?`, workspaceID)
	return err
}
