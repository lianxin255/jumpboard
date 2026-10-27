package appt

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConf writes a configuration file with a distinct mtime each call, so
// the watch can tell the edits apart.
func writeConf(t *testing.T, dir, body string, at time.Time) {
	t.Helper()
	path := filepath.Join(dir, "nginx.conf")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// statConf reads the file's info the way the poll loop does.
func statConf(t *testing.T, dir string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(filepath.Join(dir, "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

// TestConfWatchReportsForeignEdits pins the watch's purpose: an edit the
// app did not make is reported once, not once per tick.
func TestConfWatchReportsForeignEdits(t *testing.T) {
	dir := t.TempDir()
	base := time.Unix(1700000000, 0)
	writeConf(t, dir, "v1", base)
	w := newConfWatch(dir, nil)

	if w.record(statConf(t, dir), base.Add(time.Second)) {
		t.Fatal("the first look at a file nobody wrote since startup is the baseline")
	}
	writeConf(t, dir, "v2", base.Add(10*time.Second))
	if !w.record(statConf(t, dir), base.Add(11*time.Second)) {
		t.Fatal("a foreign edit should report")
	}
	if w.record(statConf(t, dir), base.Add(12*time.Second)) {
		t.Fatal("the same edit reports once, not per tick")
	}
	if !w.pending() {
		t.Error("the change waits for an answer")
	}
}

// TestConfWatchIgnoresItsOwnWrites pins the suppression: the app writing
// the file is the normal case, and must never raise the flag.
func TestConfWatchIgnoresItsOwnWrites(t *testing.T) {
	dir := t.TempDir()
	base := time.Unix(1700000000, 0)
	writeConf(t, dir, "v1", base)
	w := newConfWatch(dir, nil)
	w.record(statConf(t, dir), base.Add(time.Second))

	writeConf(t, dir, "v2", base.Add(30*time.Second))
	w.noteSelfWrite()
	if w.record(statConf(t, dir), base.Add(31*time.Second)) {
		t.Fatal("the app's own write must not report")
	}
	if w.pending() {
		t.Error("no flag from our own write")
	}
}

// TestConfWatchAnsweredStopsRetelling checks the dismiss path: answering a
// change clears it, and the same edit does not come back on the next tick.
func TestConfWatchAnsweredStopsRetelling(t *testing.T) {
	dir := t.TempDir()
	base := time.Unix(1700000000, 0)
	writeConf(t, dir, "v1", base)
	w := newConfWatch(dir, nil)
	w.record(statConf(t, dir), base.Add(time.Second))

	writeConf(t, dir, "v2", base.Add(10*time.Second))
	if !w.record(statConf(t, dir), base.Add(11*time.Second)) {
		t.Fatal("the foreign edit reports")
	}
	w.answered()
	if w.pending() {
		t.Fatal("answering clears the flag")
	}
	if w.record(statConf(t, dir), base.Add(12*time.Second)) {
		t.Fatal("an answered edit stays answered")
	}
}

// TestConfWatchMissingFileIsQuiet: a configuration never exported yet is
// the state before the first apply, and polling it says nothing.
func TestConfWatchMissingFileIsQuiet(t *testing.T) {
	w := newConfWatch(t.TempDir(), nil)
	if w.record(nil, time.Now()) {
		t.Fatal("no file, no report")
	}
}
