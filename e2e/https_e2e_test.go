package e2e

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
)

// context3s is the short deadline every probe in this file runs under.
func context3s(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 3*time.Second)
}

// writeTestCert writes a self-signed certificate for secure.example.com,
// loopback included, the same shape the engine's own tests use.
func writeTestCert(t *testing.T, certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "secure.example.com"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"secure.example.com", "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatal(err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatal(err)
	}
}

// tlsGet fetches a URL over TLS without verifying the self-signed
// certificate, asking for serverName in the handshake.
func tlsGet(t *testing.T, port int, serverName, path string) (*http.Response, string, error) {
	t.Helper()
	ctx, cancel := context3s(t)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Transport: &http.Transport{
		Proxy:           nil,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: serverName},
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp, string(b), err
}

// noFollowGet fetches a URL and does not follow redirects, so a test can
// read the redirect itself.
func noFollowGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	ctx, cancel := context3s(t)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return client.Do(req)
}

// canBind80 reports whether this process may open port 80, which the
// redirect listener needs and a normal user may not have.
func canBind80(t *testing.T) bool {
	t.Helper()
	ln, err := net.Listen("tcp", ":80")
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// TestHTTPSMappingEndToEnd walks an HTTPS rule: the app terminates TLS
// with the mapping's own certificate and, when the machine lets it hold
// port 80, sends plain HTTP over to the TLS port.
func TestHTTPSMappingEndToEnd(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()

	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	writeTestCert(t, certPath, keyPath)

	listen := freePort(t)
	created, err := a.Store.CreateMapping(model.Mapping{
		Name: "安全站", Protocol: model.HTTPS, Enabled: true,
		ListenPort: listen, ServerNames: "secure.example.com", ListenPath: "/",
		UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		TLSCertPath: certPath, TLSKeyPath: keyPath,
		RedirectHTTP: true, ProxyMode: model.ProxyStandard,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Probed before the apply: once the engine holds port 80, a probe
	// would only find the engine's own listener.
	canRedirect := canBind80(t)
	if err := a.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	waitPort(t, listen, true)

	// The request is TLS the whole way: the answer comes with a session,
	// and its body is the upstream's.
	resp, body, err := tlsGet(t, listen, "secure.example.com", "/")
	if err != nil {
		t.Fatalf("request over TLS: %v", err)
	}
	if resp.TLS == nil {
		t.Error("the answer did not come over TLS")
	}
	if body != "upstream-ok" {
		t.Errorf("body = %q, want upstream-ok", body)
	}
	if listening, problem := a.RuleState(created.ID); !listening {
		t.Errorf("the rule should be listening: %s", problem)
	}

	// The redirect depends on a privilege the machine may not grant: when
	// port 80 can be bound it answers 301, and when it cannot the apply
	// says so instead of failing the whole rule set.
	if canRedirect {
		r, err := noFollowGet(t, "http://127.0.0.1:80/page?x=1")
		if err != nil {
			t.Fatalf("the redirect port: %v", err)
		}
		defer r.Body.Close()
		if r.StatusCode != http.StatusMovedPermanently {
			t.Errorf("status = %d, want 301", r.StatusCode)
		}
		if loc := r.Header.Get("Location"); !strings.HasPrefix(loc, "https://127.0.0.1/page?x=1") {
			t.Errorf("Location = %q", loc)
		}
	} else {
		if warn := a.ApplyWarning(); !strings.Contains(warn, "跳转") {
			t.Errorf("an unbindable port 80 should warn: %q", warn)
		}
		// The warning must not have taken the TLS rule down.
		if _, body, err := tlsGet(t, listen, "secure.example.com", "/"); err != nil || body != "upstream-ok" {
			t.Errorf("the TLS rule should survive the redirect warning: %q %v", body, err)
		}
	}
}

// TestHTTPSExportHonorsListenPort checks that a rule on a port other than
// 443 exports as that port, and that the export reads back into the rule
// it came from.
func TestHTTPSExportHonorsListenPort(t *testing.T) {
	a := testApp(t)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	writeTestCert(t, certPath, keyPath)

	listen := freePort(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "端口站", Protocol: model.HTTPS, Enabled: true,
		ListenPort: listen, ServerNames: "port.example.com", ListenPath: "/",
		UpstreamHost: "10.0.0.8", UpstreamPort: 8443,
		TLSCertPath: certPath, TLSKeyPath: keyPath, RedirectHTTP: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(a.Paths.ExportDir, "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	conf := string(data)
	if !strings.Contains(conf, fmt.Sprintf("listen %d ssl;", listen)) {
		t.Errorf("the export should use the mapping's port:\n%s", conf)
	}
	rules, err := nginx.Parse(conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("the export should read back as one rule: %+v", rules)
	}
	r := rules[0]
	if r.Protocol != model.HTTPS || r.ListenPort != listen || !r.RedirectHTTP {
		t.Errorf("round trip lost details: %+v", r)
	}
}
