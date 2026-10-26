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
// The production path is AskAIMulti, which the UI calls from a goroutine;
// this hook runs the same one, so what the tests drive is what ships.

// RunAISync asks the model and keeps the first usable draft, without the
// UI.
func (a *App) RunAISync(sentence string) error {
	_, drafts, err := a.AskAIMulti(sentence)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAIDraft = nil
	if err == nil && len(drafts) > 0 {
		m := drafts[0].Mapping
		a.lastAIDraft = &m
	}
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
