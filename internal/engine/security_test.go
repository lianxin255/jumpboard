package engine

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The tests below lock the review findings that no existing test covered:
// each one failed before the fix, and each is a behavior a user relies on.

// TestWhitelistBlocksStreamRules checks the address filter on TCP and UDP.
//
// It used to apply to HTTP only, so turning the whitelist on closed the web
// rules while leaving SSH and database forwards open to every address.
func TestWhitelistBlocksStreamRules(t *testing.T) {
	upPort, stop := tcpEchoServer2(t)
	defer stop()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{
		WhitelistIPs: []string{"10.9.9.9"}, // loopback is not in it
		Mappings: []model.Mapping{{
			ID: "1", Name: "tcp", Protocol: model.TCP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(listen), time.Second)
	if err == nil {
		// Connected: the rule must hang up on us without forwarding.
		_ = c.SetDeadline(time.Now().Add(500 * time.Millisecond))
		_, _ = c.Write([]byte("ping\n"))
		buf := make([]byte, 16)
		n, rerr := c.Read(buf)
		c.Close()
		if n > 0 && rerr == nil {
			t.Errorf("a whitelisted-out client was forwarded: %q", buf[:n])
		}
	}
}

// TestUDPBadUpstreamDoesNotPanic checks that a rule whose upstream cannot
// be resolved drops datagrams instead of writing to a nil socket, which
// used to panic the whole process from a goroutine.
func TestUDPBadUpstreamDoesNotPanic(t *testing.T) {
	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "udp", Protocol: model.UDP, Enabled: true,
		ListenPort: listen,
		// A name that cannot resolve: the session has no socket.
		UpstreamHost: "no-such-host.invalid..x", UpstreamPort: 53,
	}}}); err != nil {
		t.Fatal(err)
	}

	c, err := net.Dial("udp", "127.0.0.1:"+itoa(listen))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// The datagram reaches readClients, which must survive.
	_, _ = c.Write([]byte("hello"))
	time.Sleep(150 * time.Millisecond)

	// Still alive: the process did not panic, and the rule still runs.
	if e.Status() != Running {
		t.Errorf("the engine should still be running: %s", e.Status())
	}
}

// TestSamePortMixedProtocolsIsRefused checks that an HTTPS rule sharing a
// port with an HTTP rule is reported instead of being served as plain HTTP.
func TestSamePortMixedProtocolsIsRefused(t *testing.T) {
	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()
	res, err := e.Apply(Spec{Mappings: []model.Mapping{
		{ID: "1", Name: "plain", Protocol: model.HTTP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{ID: "2", Name: "tls", Protocol: model.HTTPS, Enabled: true,
			ListenPort: listen, ServerNames: "x.example.com",
			UpstreamHost: "127.0.0.1", UpstreamPort: 2,
			TLSCertPath: "/nonexistent/cert.pem", TLSKeyPath: "/nonexistent/key.pem"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "tls") && strings.Contains(w, "没有生效") {
			found = true
		}
	}
	if !found {
		t.Errorf("the TLS rule sharing a port must be reported: %v", res.Warnings)
	}
}

// TestRetargetKeepsThePort checks that pointing a TCP rule at a different
// backend applies: the old listener holds the port, so reopening it used to
// fail with "address already in use".
func TestRetargetKeepsThePort(t *testing.T) {
	first, stopFirst := tcpEchoServer2(t)
	defer stopFirst()
	second, stopSecond := tcpEchoServer2(t)
	defer stopSecond()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	spec := Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "tcp", Protocol: model.TCP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: first,
	}}}
	if _, err := e.Apply(spec); err != nil {
		t.Fatal(err)
	}

	spec.Mappings[0].UpstreamPort = second
	res, err := e.Apply(spec)
	if err != nil {
		t.Fatalf("retargeting a running rule should apply: %v", err)
	}
	if len(res.Retargeted) == 0 {
		t.Error("the port should be reported as retargeted")
	}
	if _, err := dialEcho2(listen); err != nil {
		t.Errorf("the rule should still forward: %v", err)
	}
}

// TestBadCertificateKeepsTheOldRule checks the transaction: a rule whose
// certificate cannot be read must not take the running site down.
func TestBadCertificateKeepsTheOldRule(t *testing.T) {
	up := httpTestServer(t)
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	spec := Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "web", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up),
		ListenPath: "/",
	}}}
	if _, err := e.Apply(spec); err != nil {
		t.Fatal(err)
	}
	before := httpGet2(t, listen)
	if before == "" {
		t.Fatal("the rule should serve before the bad edit")
	}

	// The same port, now TLS with a certificate that is not there.
	spec.Mappings[0].Protocol = model.HTTPS
	spec.Mappings[0].TLSCertPath = "/nonexistent/cert.pem"
	spec.Mappings[0].TLSKeyPath = "/nonexistent/key.pem"
	if _, err := e.Apply(spec); err == nil {
		t.Fatal("an unreadable certificate should fail the apply")
	}
	// The HTTP rule keeps serving: the failed apply changed nothing.
	if after := httpGet2(t, listen); after == "" {
		t.Error("the running rule was taken down by a failed apply")
	}
}

// TestCheckDoesNotFlagItsOwnPorts checks the preflight against a running
// engine: the ports it holds are not "taken" for the purpose of "would
// applying work".
func TestCheckDoesNotFlagItsOwnPorts(t *testing.T) {
	upPort, stop := tcpEchoServer2(t)
	defer stop()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	spec := Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "tcp", Protocol: model.TCP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
	}}}
	if _, err := e.Apply(spec); err != nil {
		t.Fatal(err)
	}
	if err := e.Check(spec); err != nil {
		t.Errorf("checking a running configuration should pass: %v", err)
	}
}

// helpers

func tcpEchoServer2(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() { _ = ln.Close() }
}

func dialEcho2(port int) (string, error) {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write([]byte("x\n")); err != nil {
		return "", err
	}
	buf := make([]byte, 8)
	n, err := c.Read(buf)
	return string(buf[:n]), err
}

func httpGet2(t *testing.T, port int) string {
	t.Helper()
	c := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := c.Get("http://127.0.0.1:" + itoa(port) + "/")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
