package services

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
	"github.com/lianxin255/jumpboard/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureLocalHost(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestExportImportRoundTrip(t *testing.T) {
	s := testStore(t)
	created, err := s.CreateMapping(model.Mapping{
		Name: "mysql", Protocol: model.TCP, ListenPort: 9000, Enabled: true,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddSnapshot("conf", "reload"); err != nil {
		t.Fatal(err)
	}

	p, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	// Wipe, then import into the same database.
	if err := s.Wipe(); err != nil {
		t.Fatal(err)
	}
	res, err := Import(s, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.MappingsImported != 1 || res.Skipped != 0 {
		t.Fatalf("import result: %+v", res)
	}
	list, _ := s.ListMappings()
	if len(list) != 1 || list[0].Name != "mysql" || list[0].ListenPort != 9000 {
		t.Fatalf("roundtrip mismatch: %+v", list)
	}
	if list[0].ID == created.ID {
		t.Error("导入应生成新 ID，避免与旧行冲突")
	}
}

func TestImportSanitizesHostileBackup(t *testing.T) {
	s := testStore(t)
	payload := []byte(`{
		"version": 2,
		"mappings": [
			{"name":"ok","protocol":"tcp","enabled":true,"listenPort":9000,"upstreamHost":"10.0.0.5","upstreamPort":3306},
			{"name":"bad; } server { listen 6666;","protocol":"tcp","enabled":true,"listenPort":9001,"upstreamHost":"10.0.0.6","upstreamPort":3306},
			{"name":"bad proto","protocol":"icmp","enabled":true,"listenPort":9002,"upstreamHost":"h","upstreamPort":1},
			{"name":"bad port","protocol":"tcp","enabled":true,"listenPort":99999,"upstreamHost":"h","upstreamPort":1},
			{"name":"bad proxy_pass","protocol":"http","enabled":true,"listenPort":9003,"upstreamHost":"h","upstreamPort":80,"proxyPassUrl":"file:///etc/passwd"},
			{"name":"bad url inject","protocol":"http","enabled":true,"listenPort":9004,"upstreamHost":"h","upstreamPort":80,"proxyPassUrl":"http://x/;}"}
		]
	}`)
	res, err := Import(s, payload)
	if err != nil {
		t.Fatal(err)
	}
	// Two records are unusable (an unknown protocol, an out-of-range
	// port) and are skipped. The other four import: the injected name is
	// flattened, and an invalid proxy_pass is dropped so that the mapping
	// falls back to host:port.
	if res.MappingsImported != 4 || res.Skipped != 2 {
		t.Fatalf("应导入 4 条、跳过 2 条，实际导入 %d 跳过 %d", res.MappingsImported, res.Skipped)
	}
	list, _ := s.ListMappings()
	for _, m := range list {
		if m.Name == "bad; } server { listen 6666;" {
			t.Error("注入字符没被清洗")
		}
		for _, bad := range []string{";", "}", "{"} {
			if contains(m.Name, bad) {
				t.Errorf("%q 仍含 %q", m.Name, bad)
			}
		}
		if m.ProxyPassURL != "" && !model.ValidProxyPassURL(m.ProxyPassURL) {
			t.Errorf("非法 proxy_pass 未被清空: %q", m.ProxyPassURL)
		}
	}
}

func TestImportRejectsWrongVersion(t *testing.T) {
	s := testStore(t)
	if _, err := Import(s, []byte(`{"version":99,"mappings":[]}`)); err == nil {
		t.Fatal("应拒绝版本不匹配的备份")
	}
	if _, err := Import(s, []byte(`not json`)); err == nil {
		t.Fatal("应拒绝非 JSON")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestImportNginxConf walks the conf drop path: the app's own export,
// parsed back, merges in as new mappings beside what is already stored —
// unlike a backup, which replaces.
func TestImportNginxConf(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateMapping(model.Mapping{
		Name: "已有", Protocol: model.TCP, Enabled: true, ListenPort: 15001,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}

	conf := nginx.Generate([]model.Mapping{
		{Name: "数据库", Protocol: model.TCP, Enabled: true, ListenPort: 15002,
			UpstreamHost: "10.0.0.2", UpstreamPort: 5432},
		{Name: "内网站", Protocol: model.HTTP, Enabled: true, ListenPort: 15003,
			ServerNames: "in.example", UpstreamHost: "10.0.0.3", UpstreamPort: 8080},
	}, nginx.GenOptions{})

	added, skipped, err := ImportNginxConf(s, []byte(conf))
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 || skipped != 0 {
		t.Fatalf("added %d skipped %d, want 2/0", added, skipped)
	}
	list, _ := s.ListMappings()
	if len(list) != 3 {
		t.Fatalf("the conf should merge beside the existing mapping, have %d", len(list))
	}
	names := map[string]bool{}
	for _, m := range list {
		names[m.Name] = true
	}
	if !names["已有"] || !names["数据库"] {
		t.Errorf("merge kept the old and added the new: %v", names)
	}
}

// TestImportNginxConfRefusesRubbish keeps a dropped file from half-
// importing: nothing recognizable is an error, not an empty success.
func TestImportNginxConfRefusesRubbish(t *testing.T) {
	s := testStore(t)
	if _, _, err := ImportNginxConf(s, []byte("user nginx;\n")); err == nil {
		t.Error("a conf with no server block should be refused")
	}
	if _, _, err := ImportNginxConf(s, []byte("")); err == nil {
		t.Error("an empty conf should be refused")
	}
}
