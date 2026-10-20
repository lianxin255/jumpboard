package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestResolveUsesTheEnvRoot checks that the tests and a development run can
// put the data where they want it.
func TestResolveUsesTheEnvRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NGJ_DATA_ROOT", dir)

	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if p.DataDir != filepath.Join(dir, "data") {
		t.Errorf("DataDir = %q", p.DataDir)
	}
	if p.LogsDir != filepath.Join(dir, "data", "logs") {
		t.Errorf("LogsDir = %q", p.LogsDir)
	}
	if p.ExportDir != filepath.Join(dir, "data", "nginx-conf") {
		t.Errorf("ExportDir = %q", p.ExportDir)
	}
	if p.Fallback {
		t.Error("an explicit root is never a fallback")
	}
	for _, d := range []string{p.DataDir, p.LogsDir, p.ExportDir, filepath.Join(p.DataDir, "templates", "custom")} {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Errorf("%s should exist as a directory: %v", d, err)
		}
	}
}

// TestResolveIsIdempotent checks that a second run over an existing data
// directory succeeds, which every launch does.
func TestResolveIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NGJ_DATA_ROOT", dir)
	for i := 0; i < 3; i++ {
		if _, err := Resolve(); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}

// TestWritable checks the probe the data root uses.
func TestWritable(t *testing.T) {
	if !writable(t.TempDir()) {
		t.Error("a temp directory should be writable")
	}
	if writable(filepath.Join(t.TempDir(), "missing")) {
		t.Error("a missing directory should not read as writable")
	}
}

// TestInGoBuildDir checks the `go test` detection, whose temporary binary
// says nothing about the project.
func TestInGoBuildDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the path shape differs")
	}
	// go run builds into <work>/b001/exe/<name>, where <work> is a
	// go-build directory two levels up.
	if !inGoBuildDir("/tmp/go-build123/b001/exe/test") {
		t.Error("a go run binary should be recognized")
	}
	// The test binary itself is one, which is how this test runs.
	if exe, err := os.Executable(); err == nil && !inGoBuildDir(exe) {
		t.Logf("%s is not in a go-build directory", exe)
	}
	if inGoBuildDir("/usr/local/bin/nginxjump") {
		t.Error("an installed binary should not be")
	}
}
