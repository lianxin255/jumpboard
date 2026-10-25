package appt

import (
	"context"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The methods below exist for the end-to-end tests: they run the AI path
// synchronously, so a test does not have to drive a goroutine, and expose
// the draft the model produced.
//
// The production path is AskAI, which the UI calls from a goroutine.

// RunAISync asks the model and keeps the draft, without the UI.
func (a *App) RunAISync(sentence string) error {
	_, draft, err := a.AskAI(sentence)
	a.mu.Lock()
	a.lastAIDraft = draft
	a.mu.Unlock()
	return err
}

// AIDraftForTest returns the draft from the last RunAISync.
func (a *App) AIDraftForTest() *model.Mapping {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastAIDraft
}

// PortTaken reports whether something already listens on a port, which the
// form shows before the user saves.
func (a *App) PortTaken(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = ctx
	return !portFree(port)
}
