package services

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// A configured health-check path is a path, not a URL: it must not be able
// to move the request to another host. "@evil.example/x" concatenated onto
// the upstream used to do exactly that.
func TestCheckHTTPPathStaysOnUpstream(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/", "/health", "@evil.example/steal", "//evil.example/x", "/a?b=c"} {
		hits = 0
		if st := CheckHTTP(host, port, path, http.StatusOK, false); st != model.HealthHealthy {
			t.Errorf("path %q: status %v, want healthy", path, st)
		}
		if hits != 1 {
			t.Errorf("path %q: upstream got %d requests, want 1 — the path escaped the upstream", path, hits)
		}
	}
}
