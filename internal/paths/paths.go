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
	// DataDir holds the database, generated configs, logs and snapshots.
	DataDir string
	// PrefixDir is nginx's -p directory: conf/, logs/ and temp/ under it.
	PrefixDir string
	// ResourcesDir is where the app's shipped files are (the bundled
	// nginx and the built-in templates).
	ResourcesDir string
	// NginxBinary is the bundled nginx for this platform.
	NginxBinary string
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
	// NGJ_RESOURCES names the folder holding nginx/, for the tests and for
	// running from a directory the toolkit cannot infer.
	// The nginx path must be absolute: the engine runs nginx with its
	// prefix as the working directory, so a relative path would resolve
	// against that, not against where the app was started.
	resourceDir, err := filepath.Abs(firstNonEmpty(os.Getenv("NGJ_RESOURCES"), ""))
	if err == nil && os.Getenv("NGJ_RESOURCES") == "" {
		dir, pathErr := mygo.App.Path(mygo.PathResources)
		if pathErr != nil {
			return Paths{}, pathErr
		}
		resourceDir, err = filepath.Abs(dir)
	}
	if err != nil {
		return Paths{}, err
	}
	bin := "nginx"
	if runtime.GOOS == "windows" {
		bin = "nginx.exe"
	}
	p := Paths{
		DataDir:      filepath.Join(root, "data"),
		PrefixDir:    filepath.Join(root, "data", "nginx-home"),
		ResourcesDir: resourceDir,
		NginxBinary:  filepath.Join(resourceDir, "nginx", bin),
		Fallback:     fallback,
	}
	for _, d := range []string{
		p.DataDir,
		p.PrefixDir,
		filepath.Join(p.PrefixDir, "conf"),
		filepath.Join(p.PrefixDir, "logs"),
		filepath.Join(p.PrefixDir, "temp"),
		filepath.Join(p.DataDir, "templates", "custom"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Paths{}, fmt.Errorf("creating %s: %w", d, err)
		}
	}
	if err := p.ensureMimeTypes(); err != nil {
		return Paths{}, err
	}
	return p, nil
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ensureMimeTypes puts nginx's mime.types beside the configuration it
// includes.
//
// nginx resolves an include relative to its prefix, and a configuration
// that includes a missing file fails to load, which would make every apply
// fail with no explanation. The copy comes from the bundled nginx; when
// there is none (a user-provided nginx, or a test), the built-in list
// covers the types a reverse proxy serves.
func (p Paths) ensureMimeTypes() error {
	dst := filepath.Join(p.PrefixDir, "conf", "mime.types")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	src := filepath.Join(filepath.Dir(p.NginxBinary), "mime.types")
	if data, err := os.ReadFile(src); err == nil {
		return os.WriteFile(dst, data, 0o644)
	}
	return os.WriteFile(dst, []byte(minimalMimeTypes), 0o644)
}

// minimalMimeTypes is a short list for the types a proxy serves; nginx
// needs at least a valid types block.
const minimalMimeTypes = `types {
    text/html                     html htm shtml;
    text/css                      css;
    text/xml                      xml;
    text/plain                    txt md;
    application/javascript        js mjs;
    application/json              json map;
    application/wasm              wasm;
    application/pdf               pdf;
    application/zip               zip;
    application/octet-stream      bin exe dll iso dmg;
    image/gif                     gif;
    image/jpeg                    jpeg jpg;
    image/png                     png;
    image/svg+xml                 svg svgz;
    image/webp                    webp;
    font/woff                     woff;
    font/woff2                    woff2;
    video/mp4                     mp4;
}
`

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
