package ai

import (
	"encoding/json"
	"testing"
)

// A null field is one the model left out. Counting it as supplied makes
// the update path restore nothing, which silently clears the setting: a
// null proxyMode would drop a rule's custom headers.
func TestDraftNullFieldsAreNotPresent(t *testing.T) {
	var d Draft
	if err := json.Unmarshal([]byte(`{
		"operation":"update","targetId":"id-1","name":"redis",
		"proxyMode":null,"extraDirectives":null,"enabled":null,
		"serverNames":"api.example.com"
	}`), &d); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"proxyMode", "extraDirectives", "enabled", "group", "listenPath"} {
		if d.Has(field) {
			t.Errorf("%s reported as supplied, want absent", field)
		}
	}
	if !d.Has("serverNames") {
		t.Error("serverNames was given a real value but reported absent")
	}
	if d.Enabled != nil {
		t.Errorf("enabled = %v, want nil for a null", *d.Enabled)
	}
}
