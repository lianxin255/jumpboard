package appt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The conversation sends a summary of the scene's rules, and the settings
// page promises that summary never contains a key. A rule's proxy headers
// and extra directives are where a credential would live, so they must not
// survive the projection.
func TestMappingSummariesRedactsCredentials(t *testing.T) {
	ms := []model.Mapping{{
		ID: "id-1", Name: "内网 MySQL", Enabled: true, Protocol: model.TCP,
		ListenPort: 9000, UpstreamHost: "10.0.0.5", UpstreamPort: 3306,
		ProxyHeaders:    `proxy_set_header Authorization "Bearer sk-secret-token";`,
		ProxyPassURL:    "https://user:hunter2@internal.example/x",
		TLSCertPath:     "/etc/ssl/private/cert.pem",
		TLSKeyPath:      "/etc/ssl/private/key.pem",
		ExtraDirectives: "proxy_set_header X-Api-Key sk-another-secret;",
	}}
	b, err := json.Marshal(mappingSummaries(ms))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, secret := range []string{
		"sk-secret-token", "hunter2", "user:", "cert.pem", "key.pem",
		"sk-another-secret", "Authorization", "X-Api-Key",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("summary leaked %q: %s", secret, got)
		}
	}
	// The fields a draft has to name must still be there.
	for _, want := range []string{"id-1", "内网 MySQL", "10.0.0.5", "9000", "3306", "tcp"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary lost %q: %s", want, got)
		}
	}
}

// The diagnosis summary is the other place a rule goes to the model; it
// must not carry the raw directives either.
func TestMappingSummaryOmitsExtraDirectives(t *testing.T) {
	got := mappingSummary(model.Mapping{
		Name: "redis", Enabled: true, Protocol: model.TCP,
		ListenPort: 6379, UpstreamHost: "10.0.0.9", UpstreamPort: 6379,
		ExtraDirectives: "proxy_set_header Authorization Bearer sk-leak;",
	})
	if strings.Contains(got, "sk-leak") {
		t.Errorf("diagnosis summary leaked the directive: %s", got)
	}
}
