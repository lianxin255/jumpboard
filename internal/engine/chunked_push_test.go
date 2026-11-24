package engine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestChunkedRequestBodyStreamsThroughProxy checks a body with no
// Content-Length using a synthetic HTTP client. The real Git chunked-push
// case is in TestGitPushThroughProxyReachesLocalGitHTTPBackend; neither
// fixture is a GitLab deployment test.
func TestChunkedRequestBodyStreamsThroughProxy(t *testing.T) {
	const unit = "jumpboard-chunked-" // 18 bytes
	const size = 512 << 10
	payload := strings.Repeat(unit, size/len(unit)+1)[:size]

	var gotLen int
	var gotChunked bool
	var gotAuth string
	var gotBody string
	done := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		gotChunked = len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked"
		gotAuth = r.Header.Get("Authorization")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("upstream read: %v", err)
		}
		gotLen = len(b)
		gotBody = string(b)
		if gotLen != size {
			t.Errorf("upstream read %d bytes, want %d", gotLen, size)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer up.Close()

	listen := proxyWirePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "chunked", Name: "大包", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up.URL),
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}}}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// A body of unknown length becomes a chunked request: http.NewRequest
	// sets no Content-Length, and a reader without one cannot be measured
	// ahead of time.
	body := io.NopCloser(strings.NewReader(payload))
	req, err := http.NewRequest("POST", "http://127.0.0.1:"+itoa(listen)+"/repo.git/git-receive-pack", body)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = -1 // unknown: send chunked
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d: %s", resp.StatusCode, b)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("response read: %v", err)
	}
	<-done
	if gotBody != payload {
		t.Errorf("chunked body contents differ from the source payload")
	}
	if !gotChunked {
		t.Errorf("the upstream did not see a chunked body: TransferEncoding was buffered")
	}
	if gotLen != size {
		t.Errorf("upstream received %d bytes, want %d", gotLen, size)
	}
	if gotAuth != "Basic dXNlcjpwYXNz" {
		t.Errorf("Authorization = %q, want it passed through with the chunked body", gotAuth)
	}
}
