// Package paths decides where the app keeps its data: next to the
// executable for the green build, and beside the .app on macOS so that
// upgrading never loses it.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
)

// Paths are the directories the app uses.
type Paths struct {
	// DataDir holds the database, logs, snapshots and templates.
	DataDir string
	// LogsDir is where the engine writes its access log.
	LogsDir string
	// ExportDir is where a generated nginx.conf goes, for a user who wants
	// to take the configuration to a real nginx.
	ExportDir string
	// Fallback is set when DataDir is not next to the executable,
	// because that place is not writable.
	Fallback bool
}

// Resolve returns the app's paths, creating the ones it owns.
//
// The data root follows the old Electron build so that an existing
// data/data.db is picked up as is:
//
//	Windows        <dir of the .exe>/data
//	macOS          <the .app's folder>/data
//	go run, tests  <working directory>/data, or the value of NGJ_DATA_ROOT
func Resolve() (Paths, error) {
	root, fallback, err := dataRoot()
	if err != nil {
		return Paths{}, err
	}
	p := Paths{
		DataDir:   filepath.Join(root, "data"),
		LogsDir:   filepath.Join(root, "data", "logs"),
		ExportDir: filepath.Join(root, "data", "nginx-conf"),
		Fallback:  fallback,
	}
	for _, d := range []string{
		p.DataDir,
		p.LogsDir,
		p.ExportDir,
		filepath.Join(p.DataDir, "templates", "custom"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Paths{}, fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return p, nil
}

// dataRoot returns the folder that holds data/.
func dataRoot() (root string, fallback bool, err error) {
	if env := os.Getenv("NGJ_DATA_ROOT"); env != "" {
		return env, false, nil
	}
	exe, exeErr := os.Executable()
	if exeErr != nil || inGoBuildDir(exe) {
		wd, err := os.Getwd()
		if err != nil {
			return "", false, err
		}
		return wd, false, nil
	}
	candidate := filepath.Dir(exe)
	if runtime.GOOS == "darwin" {
		// <Name>.app/Contents/MacOS/<Name>: the data belongs beside the
		// .app, never inside it, which would break its signature and be
		// lost on upgrade.
		macOS := filepath.Dir(exe)
		contents := filepath.Dir(macOS)
		bundle := filepath.Dir(contents)
		if filepath.Base(macOS) == "MacOS" && filepath.Base(contents) == "Contents" {
			candidate = filepath.Dir(bundle)
		}
	} else if runtime.GOOS == "windows" {
		// A portable build may say where it lives.
		if dir := os.Getenv("PORTABLE_EXECUTABLE_DIR"); dir != "" {
			candidate = dir
		}
	}
	if writable(candidate) {
		return candidate, false, nil
	}
	// /Applications is read-only for a non-admin user, and so are some
	// program folders on Windows.
	dir, err := mygo.App.Path(mygo.PathAppData)
	if err != nil {
		return "", true, err
	}
	return dir, true, nil
}

// inGoBuildDir reports whether exe is a `go run`/`go test` temporary
// binary, whose folder says nothing about the project.
func inGoBuildDir(exe string) bool {
	dir := filepath.Dir(exe)
	for i := 0; i < 3; i++ {
		base := filepath.Base(dir)
		if len(base) >= 9 && base[:9] == "go-build" {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

// writable reports whether a file can be created in dir.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".ngj-write-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
