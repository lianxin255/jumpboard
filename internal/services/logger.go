package services

import (
	"fmt"
	"sync"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// maxLogs is how many entries the operation log keeps.
const maxLogs = 10000

// Logger writes the operation log to the database.
//
// Writing must never break what it logs, so failures are swallowed: a full
// disk or a locked database shows up as a missing log line, not as a
// failed operation.
type Logger struct {
	store *store.Store

	mu        sync.Mutex
	sinceTrim int
}

// NewLogger returns a logger writing to s.
func NewLogger(s *store.Store) *Logger { return &Logger{store: s} }

// Log records one entry.
func (l *Logger) Log(level model.LogLevel, source model.LogSource, message string) {
	if err := l.store.AddLog(level, source, message); err != nil {
		return
	}
	l.mu.Lock()
	l.sinceTrim++
	trim := l.sinceTrim >= 200
	if trim {
		l.sinceTrim = 0
	}
	l.mu.Unlock()
	if trim {
		_ = l.store.TrimLogs(maxLogs)
	}
}

// Info records an informational entry.
func (l *Logger) Info(format string, args ...any) {
	l.Log(model.LogInfo, model.SourceApp, sprintf(format, args...))
}

// Warn records a warning.
func (l *Logger) Warn(format string, args ...any) {
	l.Log(model.LogWarn, model.SourceApp, sprintf(format, args...))
}

// Error records an error.
func (l *Logger) Error(format string, args ...any) {
	l.Log(model.LogError, model.SourceApp, sprintf(format, args...))
}

// Tail returns the last n entries, oldest first, one "level message"
// line each, for the failure diagnosis's context.
//
// A read failure returns no lines rather than an error: the diagnosis
// degrades to less context, it does not fail — the same stance the
// write side takes.
func (l *Logger) Tail(n int) []string {
	entries, err := l.store.ListLogs(n)
	if err != nil {
		return nil
	}
	// ListLogs is newest first; a log tail reads oldest first, so the
	// lines line up with the file-based access log tail beside them.
	out := make([]string, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		out = append(out, fmt.Sprintf("[%s] %s", e.Level, e.Message))
	}
	return out
}
