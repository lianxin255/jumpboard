package e2e

import (
	"testing"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TestCloneMapping checks the clone button: a copy appears, disabled, with
// no listen port (the original still holds it).
func TestCloneMapping(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "原始", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("克隆"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the clone", func() bool {
		list, _ := a.Store.ListMappings()
		return len(list) == 2
	})
	list, _ := a.Store.ListMappings()
	var clone model.Mapping
	for _, m := range list {
		if m.Name != "原始" {
			clone = m
		}
	}
	if clone.Name != "原始 副本" {
		t.Errorf("clone name: %q", clone.Name)
	}
	if clone.Enabled {
		t.Error("a clone starts disabled: two rows on one port would not bind")
	}
	if clone.ListenPort != 0 {
		t.Errorf("a clone keeps no listen port: %d", clone.ListenPort)
	}
	if clone.UpstreamHost != "10.0.0.1" || clone.UpstreamPort != 3306 {
		t.Errorf("a clone keeps the upstream: %+v", clone)
	}
}

// TestToggleMapping checks the enable switch in the table.
func TestToggleMapping(t *testing.T) {
	a := testApp(t)
	created, err := a.Store.CreateMapping(model.Mapping{
		Name: "开关", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("启用-开关"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the switch and dirty mark", func() bool {
		m, _ := a.Store.GetMapping(created.ID)
		return !m.Enabled && a.Dirty()
	})
}

// TestHealthCheckNow checks the status cell's click.
func TestHealthCheckNow(t *testing.T) {
	a := testApp(t)
	port, stop := upstream(t)
	defer stop()

	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "健康", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "127.0.0.1", UpstreamPort: port,
		HealthCheckEnabled: true, HealthStatus: model.HealthUnknown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("检查-健康"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the check result", func() bool { return tt.HasText("健康") })
}
