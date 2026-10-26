package appt

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// confPollEvery is how often the exported configuration is checked. The
// page rereads the access log every two seconds; a person editing a file
// by hand is not faster than that.
const confPollEvery = 2 * time.Second

// confWatch is the exported nginx.conf's watchman: it notices when the file
// changes without the app being the one that wrote it, which is the only
// moment the app's rules and the file it hands out can drift apart.
//
// Its zero value is ready, so the tests drive it without the app around it.
type confWatch struct {
	path string

	mu sync.Mutex
	// selfMtime is the file's mtime right after the app itself wrote it;
	// a change to anything else is someone else's edit.
	selfMtime time.Time
	// lastMtime is the newest change already accounted for, so one edit
	// reports once, not once per tick.
	lastMtime time.Time
	// external is set when a change that is not ours was seen and nobody
	// has answered it yet.
	external bool

	onChange func()

	stop chan struct{}
	done chan struct{}
}

// record folds one look at the file into the watch, and reports whether an
// unanswered external change just landed.
func (w *confWatch) record(fi os.FileInfo, now time.Time) bool {
	if fi == nil || fi.IsDir() {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	mtime := fi.ModTime()
	if w.lastMtime.IsZero() {
		// The first look only learns what the file is; whether it changes
		// afterwards is the question.
		w.lastMtime = mtime
		return false
	}
	if mtime.Equal(w.lastMtime) {
		// Nothing new since the last look — also the steady state after an
		// external change was reported and answered.
		return false
	}
	w.lastMtime = mtime
	if mtime.Equal(w.selfMtime) {
		// The app's own write, seen for the first time.
		return false
	}
	w.external = true
	return true
}

// noteSelfWrite takes the file's current mtime as the app's own, so the
// next look does not report the app to itself. A real edit landing in the
// same clock tick as the write is missed — the file's timestamp has no
// finer grain — which is the one blind spot, and a second-long one.
func (w *confWatch) noteSelfWrite() {
	fi, err := os.Stat(w.path)
	if err != nil {
		return
	}
	w.mu.Lock()
	w.selfMtime, w.lastMtime = fi.ModTime(), fi.ModTime()
	w.mu.Unlock()
}

// pending reports an unanswered external change.
func (w *confWatch) pending() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.external
}

// answered marks the change answered: it clears the flag and stops the
// same edit from re-reporting on the next tick.
func (w *confWatch) answered() {
	fi, err := os.Stat(w.path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		w.lastMtime = fi.ModTime()
	}
	w.external = false
}

// start polls the file until stop. Its lifecycle mirrors the sampler's and
// the health checker's.
func (w *confWatch) start() {
	w.Stop()
	stop, done := make(chan struct{}), make(chan struct{})
	w.stop, w.done = stop, done
	go func() {
		defer close(done)
		t := time.NewTicker(confPollEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fi, err := os.Stat(w.path)
				if err != nil {
					continue
				}
				if w.record(fi, time.Now()) && w.onChange != nil {
					w.onChange()
				}
			}
		}
	}()
}

// Stop ends the loop and waits for its goroutine. Idempotent.
func (w *confWatch) Stop() {
	stop, done := w.stop, w.done
	w.stop, w.done = nil, nil
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}

// newConfWatch builds the watch for the app's exported configuration.
func newConfWatch(exportDir string, onChange func()) *confWatch {
	return &confWatch{path: filepath.Join(exportDir, "nginx.conf"), onChange: onChange}
}
