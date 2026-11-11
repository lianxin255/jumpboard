package appt

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// An update draft that omits a field must keep the original value. Name and
// listenPort were the two that slipped through: Mapping() substitutes a
// placeholder name, and 0 is a legal "not given" for a port, so neither
// could be detected by comparing against the zero value.
func TestUpdateDraftKeepsOmittedNameAndPort(t *testing.T) {
	a := newTestApp(t)
	ws := a.ActiveWorkspace()
	orig, err := a.Store.CreateMapping(ws, model.Mapping{
		Name: "生产 Redis", Protocol: model.TCP, ListenPort: 6379,
		UpstreamHost: "10.0.0.7", UpstreamPort: 6379, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The model names only the upstream: everything else must survive.
	reply := "改一下上游。\n```json\n{\"drafts\":[{\"operation\":\"update\",\"targetId\":\"" + orig.ID +
		"\",\"protocol\":\"tcp\",\"upstreamHost\":\"10.0.0.8\",\"upstreamPort\":6379}]}\n```"
	drafts, _ := a.chatDraftsOf(ws, reply)
	if len(drafts) != 1 {
		t.Fatalf("got %d drafts, want 1", len(drafts))
	}
	got := drafts[0].Mapping
	if got.Name != orig.Name {
		t.Errorf("name = %q, want %q (the original)", got.Name, orig.Name)
	}
	if got.ListenPort != orig.ListenPort {
		t.Errorf("listenPort = %d, want %d (the original)", got.ListenPort, orig.ListenPort)
	}
	if got.UpstreamHost != "10.0.0.8" {
		t.Errorf("upstreamHost = %q, want the change the model asked for", got.UpstreamHost)
	}
}
