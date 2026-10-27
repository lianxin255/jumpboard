package appt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeUpdateHooks wires the app for an update the fake feed offers: the
// check finds the version, the install reports progress and finishes.
func fakeUpdateHooks(a *App, version string, installErr error) (installed *bool, relaunched *bool) {
	installed, relaunched = new(bool), new(bool)
	a.CheckUpdate = func(context.Context) (*UpdateInfo, error) {
		if version == "" {
			return nil, nil
		}
		return &UpdateInfo{Version: version, Notes: "### 摘要\n修复了若干问题。"}, nil
	}
	a.InstallUpdate = func(_ context.Context, progress func(float64)) error {
		progress(-1)
		progress(0.5)
		*installed = true
		return installErr
	}
	a.RelaunchApp = func() { *relaunched = true }
	return installed, relaunched
}

// lastToast is the status line the app currently shows.
func lastToast(a *App) Status {
	_, _, _, _, _, _, _, st := a.Snapshot()
	return st
}

// TestCheckUpdateFindsAndKeeps pins what a check does to the state: a find
// is kept for the settings page, up-to-date says so, and a failure keeps
// what was found before rather than clearing it.
func TestCheckUpdateFindsAndKeeps(t *testing.T) {
	a := newTestApp(t)
	if a.UpdatesEnabled() {
		t.Fatal("a test app wires no updater, so updates are not enabled")
	}
	fakeUpdateHooks(a, "1.3.0", nil)

	if err := a.checkUpdateNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	up := a.UpdateAvail()
	if up == nil || up.Version != "1.3.0" || !strings.Contains(up.Notes, "修复了若干问题") {
		t.Fatalf("the find should be kept: %+v", up)
	}

	// A failed check keeps the earlier find.
	a.CheckUpdate = func(context.Context) (*UpdateInfo, error) {
		return nil, errors.New("the feed is unreachable")
	}
	if err := a.checkUpdateNow(context.Background()); err == nil {
		t.Fatal("the failed check should report its error")
	}
	if up = a.UpdateAvail(); up == nil || up.Version != "1.3.0" {
		t.Fatalf("a failed check must keep the earlier find: %+v", up)
	}
	if a.UpdateBusy() {
		t.Error("the check is over")
	}
}

// TestCheckUpdateUpToDateToasts covers the quiet outcome: no find, a toast.
func TestCheckUpdateUpToDateToasts(t *testing.T) {
	a := newTestApp(t)
	fakeUpdateHooks(a, "", nil)

	if err := a.checkUpdateNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.UpdateAvail() != nil {
		t.Fatal("up to date should not be a find")
	}
	if st := lastToast(a); st.Text != "已是最新版本" || st.IsErr {
		t.Fatalf("toast = %+v", st)
	}
}

// TestInstallUpdateRunsAndRestarts drives the install itself: the progress
// it reports lands where the page reads it, and a successful install ends
// in the restart.
func TestInstallUpdateRunsAndRestarts(t *testing.T) {
	a := newTestApp(t)
	installed, relaunched := fakeUpdateHooks(a, "1.3.0", nil)
	if err := a.checkUpdateNow(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := a.installNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !*installed || !*relaunched {
		t.Fatalf("installed=%v relaunched=%v, both should have run", *installed, *relaunched)
	}
	if p := a.UpdateProgress(); p != 0.5 {
		t.Errorf("progress = %v, want the last report", p)
	}
	if a.UpdateBusy() {
		t.Error("the install is over")
	}
}

// TestInstallUpdateFailureSkipsRestart checks the other side: an install
// that fails reports its error and does not restart into nothing.
func TestInstallUpdateFailureSkipsRestart(t *testing.T) {
	a := newTestApp(t)
	_, relaunched := fakeUpdateHooks(a, "1.3.0", errors.New("the download broke"))
	if err := a.checkUpdateNow(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := a.installNow(context.Background()); err == nil {
		t.Fatal("the install should report its error")
	}
	if *relaunched {
		t.Error("a failed install must not restart")
	}
	if a.UpdateBusy() {
		t.Error("the install is over")
	}
}
