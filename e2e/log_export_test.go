package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestLogExportBuildsTheFiles checks the app side of the log export: the
// operation log becomes text oldest-first, and the access log is the file
// as it stands. The save dialog is a UI gesture; the content is what the
// app owns.
func TestLogExportBuildsTheFiles(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "日志导出的站")); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}

	// Two log entries the export must order oldest first.
	a.Log.Info("第一条")
	a.Log.Warn("第二条")

	data, n, err := a.AppLogExport()
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("the export should carry the log entries, got %d", n)
	}
	text := string(data)
	if i, j := strings.Index(text, "第一条"), strings.Index(text, "第二条"); i < 0 || j < 0 || i > j {
		t.Fatalf("entries should export oldest first:\n%s", text)
	}
	if !strings.Contains(text, "WARN") {
		t.Errorf("the level should travel with the message:\n%s", text)
	}

	out := filepath.Join(t.TempDir(), "logs.txt")
	if err := a.WriteExportFile(out, data); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(out); err != nil || string(got) != text {
		t.Fatalf("the written log: %v", err)
	}
}

// TestAccessLogExportReadsTheFile pushes one request through a rule and
// checks the export carries the access line the engine wrote.
func TestAccessLogExportReadsTheFile(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "访问站", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)
	if _, err := httpGet(t, "http://127.0.0.1:"+itoa(listen)+"/"); err != nil {
		t.Fatal(err)
	}

	data, err := a.AccessLogExport()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "GET /") {
		t.Fatalf("the access log export should carry the request line:\n%s", data)
	}
}
