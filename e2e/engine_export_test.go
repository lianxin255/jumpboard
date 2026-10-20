package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/nginx"
)

// TestExportWritesAnNginxConfig checks that the rules can be taken to a
// real nginx: applying writes a configuration next to the database, and
// nginx accepts it.
func TestExportWritesAnNginxConfig(t *testing.T) {
	a := testApp(t)
	up, stop := upstream(t)
	defer stop()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "导出用", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, ServerNames: "export.example.com",
		UpstreamHost: "127.0.0.1", UpstreamPort: up, ListenPath: "/",
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(a.Paths.ExportDir, "nginx.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the export should exist at %s: %v", path, err)
	}
	conf := string(data)
	listenText := itoa(listen)
	for _, want := range []string{"server {", "listen " + listenText, "server_name export.example.com", "proxy_pass"} {
		if !strings.Contains(conf, want) {
			t.Errorf("the export should contain %q:\n%s", want, conf)
		}
	}
	// The export parses back into the same rule.
	rules, err := nginx.Parse(conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "导出用" {
		t.Errorf("the export should describe the rule: %+v", rules)
	}
}

// TestRuleStateIsVisible checks the per-row state the table shows.
func TestRuleStateIsVisible(t *testing.T) {
	a := testApp(t)
	listen := freePort(t)
	created, err := a.Store.CreateMapping(model.Mapping{
		Name: "状态", Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if listening, problem := a.RuleState(created.ID); !listening {
		t.Errorf("an applied rule should be listening: %q", problem)
	}

	if err := a.StopEngine(); err != nil {
		t.Fatal(err)
	}
	if listening, _ := a.RuleState(created.ID); listening {
		t.Error("a stopped engine should have no listening rule")
	}
}
