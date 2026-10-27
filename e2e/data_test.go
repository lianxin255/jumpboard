package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestExportImportThroughFiles writes a backup, imports it into a second
// app, and checks the mappings survive.
func TestExportImportThroughFiles(t *testing.T) {
	source := testApp(t)
	if _, err := source.Store.CreateMapping(source.ActiveWorkspace(), model.Mapping{
		Name: "要备份的", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	payload, err := source.Export()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.json")
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	target := testApp(t)
	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Import(read); err != nil {
		t.Fatal(err)
	}
	list, err := target.Store.ListMappings(target.ActiveWorkspace())
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Name != "要备份的" || list[0].ListenPort != 9000 {
		t.Errorf("the mapping did not survive: %+v", list[0])
	}
}

// TestImportHostileBackupThroughApp feeds the app a backup written to
// attack it.
func TestImportHostileBackupThroughApp(t *testing.T) {
	a := testApp(t)
	hostile := []byte(`{
		"version": 2,
		"mappings": [
			{"name":"ok","protocol":"tcp","enabled":true,"listenPort":9000,"upstreamHost":"10.0.0.5","upstreamPort":3306},
			{"name":"x; } server { listen 6666;","protocol":"tcp","enabled":true,"listenPort":9001,"upstreamHost":"10.0.0.6","upstreamPort":3306},
			{"name":"bad","protocol":"icmp","enabled":true,"listenPort":9002,"upstreamHost":"127.0.0.1","upstreamPort":1},
			{"name":"inject","protocol":"http","enabled":true,"listenPort":9003,"upstreamHost":"127.0.0.1","upstreamPort":80,
			 "proxyPassUrl":"http://x/;}"}
		]
	}`)
	if err := a.Import(hostile); err != nil {
		t.Fatal(err)
	}
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 mappings (one skipped), got %d: %+v", len(list), list)
	}
	for _, m := range list {
		if m.Name == "x; } server { listen 6666;" {
			t.Error("the injected name was kept verbatim")
		}
		if m.ProxyPassURL != "" && m.Protocol == model.HTTP {
			t.Errorf("an invalid proxy_pass survived: %q", m.ProxyPassURL)
		}
	}

	// The imported configuration must also be one nginx accepts.
	if err := a.Apply(); err != nil {
		t.Errorf("the imported configuration should apply: %v", err)
	}
	a.Eng.Stop()
}

// TestWipeRebuilds checks that clearing the data leaves a working app.
func TestWipeRebuilds(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "x", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := a.Wipe(); err != nil {
		t.Fatal(err)
	}
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 0 {
		t.Fatalf("after a wipe: %v %v", list, err)
	}
	// The app still works: a mapping can be added and applied.
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "新", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatalf("applying after a wipe: %v", err)
	}
	a.Eng.Stop()
}
