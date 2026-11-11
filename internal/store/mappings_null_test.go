package store

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// A rule from before a column existed stores NULL, and the conflict guard
// must still recognise it as unchanged rather than answering "conflict".
func TestUpdateMappingIfUnchangedWithNullColumns(t *testing.T) {
	s := open(t)
	if _, err := s.CreateMapping(DefaultWorkspaceID, model.Mapping{
		Name: "legacy", Protocol: model.TCP, ListenPort: 9100,
		UpstreamHost: "10.0.0.1", UpstreamPort: 22, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := s.db.QueryRow(`SELECT id FROM mappings`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-migration row: the optional columns are NULL.
	if _, err := s.db.Exec(`UPDATE mappings SET server_names=NULL, listen_path=NULL,
		tls_cert_path=NULL, tls_key_path=NULL, extra_directives=NULL,
		proxy_headers=NULL, proxy_pass_url=NULL, health_check_target_path=NULL,
		health_check_expected_status=NULL, listen_port=NULL WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	original, err := s.GetMapping(id)
	if err != nil {
		t.Fatal(err)
	}
	proposed := original
	proposed.Name = "renamed"
	if err := s.UpdateMappingIfUnchanged(proposed, original); err != nil {
		t.Fatalf("guard rejected an unchanged NULL row: %v", err)
	}
	after, err := s.GetMapping(id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != "renamed" {
		t.Errorf("name = %q, want renamed", after.Name)
	}
}
