package appt

import "context"

// UpdateInfo is a newer version the check found, for the settings page.
// The notes are Markdown — the section of the version in CHANGELOG.md.
type UpdateInfo struct {
	Version string
	Notes   string
}

// UpdatesEnabled reports whether this build can update itself. Development
// builds and builds without updates configured cannot: the settings page
// then says so instead of offering the buttons.
func (a *App) UpdatesEnabled() bool { return a.CheckUpdate != nil }

// UpdateBusy reports whether a check or an install is running, so the
// buttons hold still while one does.
func (a *App) UpdateBusy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updateBusy
}

// UpdateAvail returns the newer version the last check found, nil when the
// app is up to date or nothing was checked yet.
func (a *App) UpdateAvail() *UpdateInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.updateAvail == nil {
		return nil
	}
	up := *a.updateAvail
	return &up
}

// UpdateProgress is how far the download of the update got, from 0 to 1,
// negative while the size is unknown or nothing downloads.
func (a *App) UpdateProgress() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updateProgress
}

// CheckForUpdates asks the feed for a newer version, off the main thread.
// An app that is up to date is news worth a toast; a failure is one too.
func (a *App) CheckForUpdates() {
	if !a.UpdatesEnabled() {
		return
	}
	a.RunBackground(func() error { return a.checkUpdateNow(context.Background()) })
}

// checkUpdateNow is the check itself, separated for the tests, which run
// it without the goroutine. A failed check keeps the last finding: what
// was found before stays offered.
func (a *App) checkUpdateNow(ctx context.Context) error {
	a.mu.Lock()
	a.updateBusy = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.updateBusy = false
		a.mu.Unlock()
	}()
	up, err := a.CheckUpdate(ctx)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.updateAvail = up
	a.mu.Unlock()
	if up == nil {
		a.Info("已是最新版本")
	}
	return nil
}

// InstallFoundUpdate downloads what the check found and, when it installed,
// restarts the app into it. The restart goes through the app's own quit
// path, which stops the engine and closes the database first.
func (a *App) InstallFoundUpdate() {
	if a.InstallUpdate == nil {
		a.Fail("此构建不支持自动更新")
		return
	}
	a.RunBackground(func() error { return a.installNow(context.Background()) })
}

// installNow is the install itself, separated like checkUpdateNow. The
// progress it reports is a fraction, negative when the size is unknown.
func (a *App) installNow(ctx context.Context) error {
	a.mu.Lock()
	a.updateBusy = true
	a.updateProgress = -1
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.updateBusy = false
		a.mu.Unlock()
	}()
	err := a.InstallUpdate(ctx, func(fraction float64) {
		a.mu.Lock()
		a.updateProgress = fraction
		a.mu.Unlock()
		a.notify()
	})
	if err != nil {
		return err
	}
	// Relaunch is posted, not waited for: this goroutine finishes and lets
	// the quit path's wait for it pass, and the app starts again.
	if a.RelaunchApp != nil {
		a.RelaunchApp()
	}
	return nil
}
