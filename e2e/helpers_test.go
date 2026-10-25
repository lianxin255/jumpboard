// Package e2e drives the whole app the way a user does: real store, real
// view, real forwarding engine. The UI tests use ui.Tester, which needs no
// window, so they run on a locked screen and in CI; the engine tests start
// the real listeners and push traffic through them.
package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
)

// testApp builds an app on a temporary data directory.
//
// JUMPBOARD_DATA_ROOT scopes the data, and the paths package honors it, so the
// test never touches the user's real data.
func testApp(t *testing.T) *appt.App {
	t.Helper()
	root := t.TempDir()
	t.Setenv("JUMPBOARD_DATA_ROOT", root)
	// The system keychain needs a running app; a test reads no key.
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")

	p, err := paths.Resolve()
	if err != nil {
		t.Fatalf("resolving paths: %v", err)
	}

	a, err := appt.New(p)
	if err != nil {
		t.Fatalf("building the app: %v", err)
	}
	a.Tpl = services.NewTemplates(builtinTemplates(t), filepath.Join(p.DataDir, "templates", "custom"))
	// Close stops nginx and the checker. It runs even when the test fails,
	// which is what keeps a run from leaving ports taken.
	t.Cleanup(a.Close)
	return a
}

// bundledTemplates loads the templates the app embeds, for the tests that
// open the picker.
func builtinTemplates(t *testing.T) map[string]model.Template {
	t.Helper()
	dir := filepath.Join("..", "internal", "services", "builtin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the built-in templates: %v", err)
	}
	files := map[string][]byte{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = data
	}
	tpls, err := services.LoadBuiltin(files)
	if err != nil {
		t.Fatalf("parsing the built-in templates: %v", err)
	}
	return tpls
}
