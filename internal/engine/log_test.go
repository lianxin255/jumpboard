package engine

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TestAccessLogIsWritten checks that both kinds of rule leave a line in
// the log the app's log page reads.
func TestAccessLogIsWritten(t *testing.T) {
	dir := t.TempDir()
	up := httpTestServer(t)
	tcpUp := tcpEchoServer(t)

	listenHTTP := freePort(t)
	listenTCP := freePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{
		LogDir: dir,
		Mappings: []model.Mapping{
			{ID: "1", Name: "h", Protocol: model.HTTP, Enabled: true,
				ListenPort: listenHTTP, UpstreamHost: "127.0.0.1",
				UpstreamPort: portOfURL(t, up), ListenPath: "/"},
			{ID: "2", Name: "t", Protocol: model.TCP, Enabled: true,
				ListenPort: listenTCP, UpstreamHost: "127.0.0.1", UpstreamPort: tcpUp},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// One HTTP request and one TCP connection.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get("http://127.0.0.1:" + itoa(listenHTTP) + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	c, err := net.Dial("tcp", "127.0.0.1:"+itoa(listenTCP))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = c.Write([]byte("x"))
	buf := make([]byte, 1)
	_, _ = io.ReadFull(c, buf)
	c.Close()
	time.Sleep(150 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(dir, "access.log"))
	if err != nil {
		t.Fatalf("the access log should exist: %v", err)
	}
	log := string(data)
	if !strings.Contains(log, "GET /") {
		t.Errorf("the HTTP request should be logged:\n%s", log)
	}
	if !strings.Contains(log, "-> 127.0.0.1:"+itoa(tcpUp)) {
		t.Errorf("the TCP connection should be logged:\n%s", log)
	}
}
