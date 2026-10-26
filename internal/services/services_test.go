package services

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

func TestCheckTCP(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if got := CheckTCP("127.0.0.1", port); got != model.HealthHealthy {
		t.Errorf("open port should be healthy: %s", got)
	}
	if got := CheckTCP("127.0.0.1", 1); got != model.HealthUnhealthy {
		t.Errorf("closed port should be unhealthy: %s", got)
	}
}

func TestCheckHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusFound)
	})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	srv := &http.Server{Handler: mux}
	go srv.Serve(l)
	defer srv.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if got := CheckHTTP("127.0.0.1", port, "/ok", 200, false); got != model.HealthHealthy {
		t.Errorf("200 should be healthy: %s", got)
	}
	if got := CheckHTTP("127.0.0.1", port, "/missing", 200, false); got != model.HealthUnhealthy {
		t.Errorf("404 should be unhealthy when 200 is expected: %s", got)
	}
	if got := CheckHTTP("127.0.0.1", port, "/missing", 404, false); got != model.HealthHealthy {
		t.Errorf("404 should be healthy when 404 is expected: %s", got)
	}
	// A redirect is not followed: its status is the answer.
	if got := CheckHTTP("127.0.0.1", port, "/redirect", 302, false); got != model.HealthHealthy {
		t.Errorf("302 should be healthy when 302 is expected: %s", got)
	}
	// An empty path means "/", which this mux answers with 404.
	if got := CheckHTTP("127.0.0.1", port, "", 404, false); got != model.HealthHealthy {
		t.Errorf("an empty path should mean /: %s", got)
	}
}

func TestTailFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		b.WriteString("line ")
		b.WriteString(itoa(i))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := TailFile(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || lines[0] != "line 98" || lines[2] != "line 100" {
		t.Fatalf("tail: %v", lines)
	}

	// More lines than the file has returns the whole file.
	all, err := TailFile(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 100 {
		t.Fatalf("want 100 lines, got %d", len(all))
	}

	if got, err := TailFile(filepath.Join(t.TempDir(), "missing.log"), 5); err == nil || got != nil {
		t.Errorf("a missing file should fail: %v %v", got, err)
	}
	if got, err := TailFile(path, 0); err != nil || got != nil {
		t.Errorf("zero lines should return nothing: %v %v", got, err)
	}

	dir := filepath.Join(t.TempDir(), "not-a-log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := TailFile(dir, 5); err == nil || got != nil {
		t.Errorf("a directory must not look like an empty log: %v %v", got, err)
	}
}

func TestTemplatesCRUD(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"mysql.yml": []byte("id: mysql\nname: MySQL\ncategory: 数据库\nmapping:\n  protocol: tcp\n  upstreamPort: 3306\n"),
	}
	builtin, err := LoadBuiltin(files)
	if err != nil {
		t.Fatal(err)
	}
	tpls := NewTemplates(builtin, dir)

	list := tpls.List()
	if len(list) != 1 || list[0].ID != "mysql" || !list[0].Builtin {
		t.Fatalf("builtin list: %+v", list)
	}

	custom := model.Template{
		ID: "my-api", Name: "我的 API", Category: "Web",
		Mapping: model.TemplateMapping{Protocol: model.HTTP, ListenPort: 9000, UpstreamPort: 8080},
	}
	if err := tpls.Save(custom); err != nil {
		t.Fatal(err)
	}
	list = tpls.List()
	if len(list) != 2 {
		t.Fatalf("after save: %d templates", len(list))
	}
	var found bool
	for _, tp := range list {
		if tp.ID == "my-api" && !tp.Builtin && tp.Name == "我的 API" {
			found = true
		}
	}
	if !found {
		t.Errorf("the custom template is not listed: %+v", list)
	}

	// An ID that could escape the directory is refused.
	for _, bad := range []string{"../evil", "a/b", "", strings.Repeat("x", 65)} {
		if err := tpls.Save(model.Template{ID: bad, Name: "x", Mapping: model.TemplateMapping{Protocol: model.TCP}}); err == nil {
			t.Errorf("id %q should be refused", bad)
		}
	}

	if err := tpls.Delete("my-api"); err != nil {
		t.Fatal(err)
	}
	if got := len(tpls.List()); got != 1 {
		t.Errorf("after delete: %d templates", got)
	}
	// Deleting twice is not an error.
	if err := tpls.Delete("my-api"); err != nil {
		t.Errorf("deleting a gone template should be fine: %v", err)
	}
}

func TestHealthCheckerRunOnce(t *testing.T) {
	s := testStore(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	upPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	live, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	livePort := live.Addr().(*net.TCPAddr).Port

	if _, err := s.CreateMapping(model.Mapping{
		Name: "up", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "127.0.0.1", UpstreamPort: livePort,
		HealthCheckEnabled: true, HealthStatus: model.HealthUnknown,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMapping(model.Mapping{
		Name: "down", Protocol: model.TCP, Enabled: true, ListenPort: 9001,
		UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		HealthCheckEnabled: true, HealthStatus: model.HealthUnknown,
	}); err != nil {
		t.Fatal(err)
	}
	// A mapping with checks off is left alone.
	if _, err := s.CreateMapping(model.Mapping{
		Name: "off", Protocol: model.TCP, Enabled: true, ListenPort: 9002,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		HealthCheckEnabled: false, HealthStatus: model.HealthUnknown,
	}); err != nil {
		t.Fatal(err)
	}

	c := NewChecker(s)
	changed := c.RunOnce()
	if len(changed) != 2 {
		t.Fatalf("want 2 changed, got %d", len(changed))
	}
	list, _ := s.ListMappings()
	status := map[string]model.HealthStatus{}
	for _, m := range list {
		status[m.Name] = m.HealthStatus
	}
	if status["up"] != model.HealthHealthy {
		t.Errorf("up should be healthy: %s", status["up"])
	}
	if status["down"] != model.HealthUnhealthy {
		t.Errorf("down should be unhealthy: %s", status["down"])
	}
	if status["off"] != model.HealthUnknown {
		t.Errorf("a mapping with checks off should be untouched: %s", status["off"])
	}

	// A second pass finds nothing changed.
	if again := c.RunOnce(); len(again) != 0 {
		t.Errorf("second pass changed %d", len(again))
	}
}

func TestHealthCheckerLoopStops(t *testing.T) {
	s := testStore(t)
	c := NewChecker(s)
	c.Interval = 20 * time.Millisecond
	done := make(chan struct{})
	c.OnResult = func([]HealthChange) { close(done) }

	if _, err := s.CreateMapping(model.Mapping{
		Name: "x", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		HealthCheckEnabled: true, HealthStatus: model.HealthUnknown,
	}); err != nil {
		t.Fatal(err)
	}
	c.Start()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the loop never reported a change")
	}
	c.Stop()
}
