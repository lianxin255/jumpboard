package services

import (
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
