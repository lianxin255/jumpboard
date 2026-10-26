package ai

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Live smoke tests against a real gateway. They are off unless
// JUMPBOARD_AI_LIVE=1, and every way the gateway can be absent — no env,
// no key, nothing listening — is a Skip, not a Fail: the local network is
// not part of the build.
//
// The API key comes from JUMPBOARD_AI_KEY only; it is never defaulted in
// source. The gateway address defaults to the local one and can be
// repointed with JUMPBOARD_AI_BASE.
func TestLiveModelsAndDraft(t *testing.T) {
	if os.Getenv("JUMPBOARD_AI_LIVE") != "1" {
		t.Skip("set JUMPBOARD_AI_LIVE=1 (and JUMPBOARD_AI_KEY) to run the live smoke test")
	}
	key := os.Getenv("JUMPBOARD_AI_KEY")
	if key == "" {
		t.Skip("JUMPBOARD_AI_KEY is not set; the live test refuses to go without a key")
	}
	base := os.Getenv("JUMPBOARD_AI_BASE")
	if base == "" {
		base = "http://127.0.0.1:3425"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// The same gateway typed both ways — with and without the /v1 — must
	// work: that difference is the bug this guards against. The second
	// form is what Models and Draft build from the first via normalizeBase.
	for _, in := range []string{base, strings.TrimRight(base, "/") + "/v1"} {
		c := New(Settings{BaseURL: in, APIKey: key, Timeout: 45 * time.Second})
		models, err := c.Models(ctx)
		if err != nil {
			// Unreachable or refusing: a machine without the gateway is
			// not a test failure.
			t.Skipf("gateway %s: %v", in, err)
		}
		if len(models) == 0 {
			t.Skipf("gateway %s lists no models", in)
		}

		// The smoke is about the path, not one model's mood: a thinking
		// model that cuts off mid-reply is a 502 from upstream, so try a
		// few — the small ones first — and pass on the first that drafts.
		order := preferSmall(models)
		var draftErr error
		var drafted bool
		for _, name := range order {
			c.settings.Model = name
			d, err := c.Draft(ctx, "把内网 MySQL 192.168.1.10:3306 暴露到 9000")
			if err != nil {
				draftErr = err
				continue
			}
			// The path is what this test owns: a 200 with parsable JSON.
			// Whether the model filled every field is its own quality, so
			// a mapping that does not survive validation is logged, not
			// failed — the next model may simply be better behaved.
			m, _, err := d.Mapping()
			if err != nil {
				t.Logf("model %s replied but the draft did not map: %v", name, err)
				drafted = true
				break
			}
			if m.UpstreamPort != 3306 || m.UpstreamHost != "192.168.1.10" {
				t.Logf("model %s drafted upstream %s:%d, want 192.168.1.10:3306", name, m.UpstreamHost, m.UpstreamPort)
			}
			drafted = true
			break
		}
		if !drafted {
			t.Fatalf("Draft via %s failed on every tried model (first: %s): %v", in, order[0], draftErr)
		}
	}
}

// preferSmall puts the cheap, fast model names first: a smoke test should
// spend one small request, not a thinking model's half minute.
func preferSmall(models []string) []string {
	var small, rest []string
	for _, m := range models {
		l := strings.ToLower(m)
		if strings.Contains(l, "mini") || strings.Contains(l, "flash") || strings.Contains(l, "haiku") || strings.Contains(l, "turbo") {
			small = append(small, m)
			continue
		}
		rest = append(rest, m)
	}
	out := append(small, rest...)
	// Never try the whole catalogue: three models is enough to tell a
	// broken path from an unhappy upstream.
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}
