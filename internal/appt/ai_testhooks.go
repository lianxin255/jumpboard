package appt

import (
	"context"
	"time"
)

// The method below exists for the end-to-end tests: it answers a question
// the form asks on its own thread, synchronously, so a test does not have
// to drive a goroutine.

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
