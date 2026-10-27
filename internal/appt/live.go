package appt

import (
	"sync"
	"time"

	"github.com/lianxin255/jumpboard/internal/engine"
)

// rulePollEvery is the watcher's fallback: with no event either — an idle
// engine pokes nothing — the snapshot is still reread this often, so a
// change the events somehow missed cannot outlive it.
const rulePollEvery = time.Second

// ruleWatcher keeps the table's rule counters live. It wakes on the
// engine's events — a connection opened, a pipe ended, an upstream refused
// — and once a second besides, copies the engine's snapshot into the app
// and asks for a frame only when what the table would show actually
// changed. The copy runs on this watcher's own goroutine, never on the
// serving goroutine that poked: a reader that ran there could block on the
// engine's lock while an apply waits for that very goroutine to leave, and
// the two would wait forever.
type ruleWatcher struct {
	eng *engine.Local
	// refresh copies the engine's snapshot into the app and reports
	// whether anything the frame draws changed.
	refresh func() bool
	// onChange asks the window for a frame.
	onChange func()

	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

// newRuleWatcher builds a watcher over eng. Both callbacks are required by
// Start; a test may leave onChange nil to only exercise the refresh.
func newRuleWatcher(eng *engine.Local, refresh func() bool, onChange func()) *ruleWatcher {
	return &ruleWatcher{eng: eng, refresh: refresh, onChange: onChange}
}

// Start begins watching. Calling it twice restarts the loop.
func (w *ruleWatcher) Start() {
	w.Stop() // a loop already running must end first
	stop, done := make(chan struct{}), make(chan struct{})
	w.mu.Lock()
	w.stop, w.done = stop, done
	w.mu.Unlock()
	go func() {
		defer close(done)
		t := time.NewTicker(rulePollEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-w.eng.Events():
			case <-t.C:
			}
			// Every pending poke says the same thing — reread the rules —
			// so they fold into this one pass.
		drain:
			for {
				select {
				case <-w.eng.Events():
				default:
					break drain
				}
			}
			if w.refresh() && w.onChange != nil {
				w.onChange()
			}
		}
	}()
}

// Stop ends the loop and waits for its goroutine to leave. Idempotent.
func (w *ruleWatcher) Stop() {
	w.mu.Lock()
	stop, done := w.stop, w.done
	w.stop, w.done = nil, nil
	w.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}

// refreshRules copies the engine's snapshot into what the frame reads and
// reports whether anything the table draws of it changed — the watcher's
// only reason to ask for a frame. The identity fields (name, addresses) are
// left out of the comparison: they move only on a re-apply, which asks for
// a frame of its own.
func (a *App) refreshRules() bool {
	status := string(a.Eng.Status())
	rules := a.Eng.Rules()
	a.mu.Lock()
	defer a.mu.Unlock()
	changed := a.engine != status || ruleSnapshotChanged(a.ruleStatus, rules)
	a.engine = status
	a.ruleStatus = rules
	return changed
}

// ruleSnapshotChanged compares what the table shows of two snapshots.
func ruleSnapshotChanged(prev, next []engine.RuleStatus) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		p, n := prev[i], next[i]
		switch {
		case p.MappingID != n.MappingID,
			p.Listening != n.Listening,
			p.Connections != n.Connections,
			p.Active != n.Active,
			p.BytesIn != n.BytesIn,
			p.BytesOut != n.BytesOut,
			!p.LastCloseAt.Equal(n.LastCloseAt),
			!p.LastErrAt.Equal(n.LastErrAt),
			p.LastErr != n.LastErr,
			p.Err != n.Err,
			p.Warning != n.Warning:
			return true
		}
	}
	return false
}
