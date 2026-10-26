package store

import (
	"path/filepath"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureLocalHost(); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestReplaceAllMappingsKeepsSnapshotHistory locks in that replacing the
// rules — what both an import and a rollback do — leaves the snapshots
// where they are. A rollback used to erase the whole history on its way
// in, which is the opposite of what a history is for.
func TestReplaceAllMappingsKeepsSnapshotHistory(t *testing.T) {
	s := open(t)
	if _, err := s.CreateMapping(model.Mapping{
		Name: "old", Protocol: model.TCP, ListenPort: 9000,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSnapshot("server { }", "apply"); err != nil {
		t.Fatal(err)
	}

	if err := s.ReplaceAllMappings([]model.Mapping{{
		Name: "new", Protocol: model.TCP, ListenPort: 9001,
		UpstreamHost: "10.0.0.6", UpstreamPort: 3306, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}

	snaps, err := s.ListSnapshots(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Reason != "apply" {
		t.Fatalf("the snapshot should have survived the replace: %+v", snaps)
	}
	list, err := s.ListMappings()
	if err != nil || len(list) != 1 || list[0].Name != "new" {
		t.Fatalf("the rules should have been replaced: %+v %v", list, err)
	}
}

func TestMappingCRUD(t *testing.T) {
	s := open(t)
	created, err := s.CreateMapping(model.Mapping{
		Name: "mysql", Protocol: model.TCP, ListenPort: 9000,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.SortOrder != 0 {
		t.Fatalf("bad create result: %+v", created)
	}
	list, err := s.ListMappings()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Name != "mysql" || !list[0].Enabled || list[0].Protocol != model.TCP {
		t.Fatalf("roundtrip mismatch: %+v", list[0])
	}

	created.Name = "mysql-renamed"
	created.ListenPort = 9100
	if err := s.UpdateMapping(created); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMapping(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "mysql-renamed" || got.ListenPort != 9100 {
		t.Fatalf("update did not stick: %+v", got)
	}

	if err := s.SetMappingEnabled(created.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMapping(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetMapping(created.ID); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestEnabledFilter(t *testing.T) {
	s := open(t)
	for _, m := range []model.Mapping{
		{Name: "a", Protocol: model.TCP, ListenPort: 1, UpstreamHost: "h", UpstreamPort: 1, Enabled: true},
		{Name: "b", Protocol: model.TCP, ListenPort: 2, UpstreamHost: "h", UpstreamPort: 1, Enabled: false},
	} {
		if _, err := s.CreateMapping(m); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := s.ListMappings()
	enabled, _ := s.ListEnabledMappings()
	if len(all) != 2 || len(enabled) != 1 || enabled[0].Name != "a" {
		t.Fatalf("all=%d enabled=%d", len(all), len(enabled))
	}
}

func TestSettings(t *testing.T) {
	s := open(t)
	if got := s.Setting(SettingTheme, "system"); got != "system" {
		t.Fatalf("default: %q", got)
	}
	if err := s.SetSetting(SettingTheme, "dark"); err != nil {
		t.Fatal(err)
	}
	if got := s.Setting(SettingTheme, "system"); got != "dark" {
		t.Fatalf("after set: %q", got)
	}
	if s.WhitelistOn() {
		t.Fatal("whitelist should default off")
	}
	if err := s.SetWhitelistOn(true); err != nil {
		t.Fatal(err)
	}
	if !s.WhitelistOn() {
		t.Fatal("whitelist should be on")
	}
}

func TestSnapshotsSameMillisecondStayNewestFirst(t *testing.T) {
	s := open(t)
	frozen := int64(1_700_000_000_000)
	prev := nowMillis
	nowMillis = func() int64 { return frozen }
	t.Cleanup(func() { nowMillis = prev })

	for _, content := range []string{"v1", "v2", "v3"} {
		if err := s.AddSnapshot(content, "reload"); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListSnapshots(10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, sn := range list {
		got = append(got, sn.Content)
	}
	want := []string{"v3", "v2", "v1"}
	if len(got) != len(want) {
		t.Fatalf("list = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("same-millisecond order = %v, want newest first %v", got, want)
		}
	}

	if err := s.TrimSnapshots(2); err != nil {
		t.Fatal(err)
	}
	kept, err := s.ListSnapshots(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || kept[0].Content != "v3" || kept[1].Content != "v2" {
		var contents []string
		for _, sn := range kept {
			contents = append(contents, sn.Content)
		}
		t.Fatalf("trim kept %v, want newest [v3 v2]", contents)
	}
}

func TestSnapshotsTrim(t *testing.T) {
	s := open(t)
	for i := 0; i < 5; i++ {
		if err := s.AddSnapshot("conf", "reload"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.TrimSnapshots(3); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListSnapshots(10)
	if len(list) != 3 {
		t.Fatalf("trim left %d", len(list))
	}
}

func TestWhitelistAndWipe(t *testing.T) {
	s := open(t)
	e, err := s.AddWhitelist("192.168.1.0/24", "lab")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetWhitelistEnabled(e.ID, false); err != nil {
		t.Fatal(err)
	}
	ips, _ := s.EnabledWhitelistIPs()
	if len(ips) != 0 {
		t.Fatalf("disabled entry still enabled: %v", ips)
	}
	if _, err := s.AddWhitelist("10.0.0.1", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Wipe(); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ListWhitelist()
	if len(after) != 0 {
		t.Fatalf("wipe left %d entries", len(after))
	}
}

// TestMappingGroupRoundtrip checks the optional group label through every
// write path: created with one, updated to another, and created without
// one — which must read back as the empty string, not NULL.
func TestMappingGroupRoundtrip(t *testing.T) {
	s := open(t)
	created, err := s.CreateMapping(model.Mapping{
		Name: "mysql", Group: "数据库", Protocol: model.TCP, ListenPort: 9000,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.ListMappings()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Group != "数据库" {
		t.Fatalf("group after create: %q", list[0].Group)
	}

	created.Group = "内网"
	if err := s.UpdateMapping(created); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMapping(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Group != "内网" {
		t.Fatalf("group after update: %q", got.Group)
	}

	bare, err := s.CreateMapping(model.Mapping{
		Name: "bare", Protocol: model.TCP, ListenPort: 9001,
		UpstreamHost: "10.0.0.5", UpstreamPort: 3306, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	bareGot, err := s.GetMapping(bare.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bareGot.Group != "" {
		t.Fatalf("mapping without a group read back %q, want \"\"", bareGot.Group)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.db")
	for i := 0; i < 3; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		s.Close()
	}
	// A reopened database must still carry every column migrate adds —
	// including group_name, which only the ALTER path ever creates.
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cols, err := s.tableColumns("mappings")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"proxy_mode", "health_check_expected_status", "group_name"} {
		if !cols[name] {
			t.Errorf("mappings.%s missing after reopen", name)
		}
	}
}
