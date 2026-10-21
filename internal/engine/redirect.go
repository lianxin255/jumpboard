package engine

import (
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// The HTTP→HTTPS redirect: one plain-HTTP listener that answers every
// request with a redirect to the same address over TLS, the way the
// generated configuration's `return 301` server block did.

// redirectProto marks a redirect listener in the running set. It is not a
// protocol a mapping can ask for; it only keys the listener so that a
// reapply leaves an unchanged one alone.
const redirectProto model.Protocol = "redirect"

// redirectForwarder answers plain HTTP with a redirect to HTTPS.
type redirectForwarder struct {
	srv     *http.Server
	ln      net.Listener
	port    int
	conns   atomic.Int64
	active  atomic.Int32
	closing atomic.Bool
}

func (f *redirectForwarder) key() ruleKey {
	return ruleKey{proto: redirectProto, listen: listenAddr(f.port)}
}

func (f *redirectForwarder) stats() (int64, int) {
	return f.conns.Load(), int(f.active.Load())
}

func (f *redirectForwarder) Close() error {
	if f.closing.Swap(true) {
		return nil
	}
	return f.srv.Close()
}

// newRedirectForwarder starts the port-80 listener that sends browsers to
// the TLS port. Like the generated configuration, the redirect carries the
// whitelist: without it port 80 would answer clients the TLS port refuses.
func newRedirectForwarder(port int, whitelist []*net.IPNet, access *accessLog) (*redirectForwarder, error) {
	ln, err := net.Listen("tcp", listenAddr(port))
	if err != nil {
		return nil, err
	}
	f := &redirectForwarder{ln: ln, port: port}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusMovedPermanently}
		if !allowed(whitelist, r.RemoteAddr) {
			http.Error(rec, "403 Forbidden", http.StatusForbidden)
			access.write(port, r, http.StatusForbidden, 0)
			return
		}
		// $host has no port, so a Host header that carries one loses it:
		// the redirect goes to the TLS port, not back to this one.
		host := stripPort(r.Host)
		if host == "" {
			host = "localhost"
		}
		target := *r.URL
		target.Scheme, target.Host = "https", host
		http.Redirect(rec, r, target.String(), http.StatusMovedPermanently)
		access.write(port, r, http.StatusMovedPermanently, rec.bytes)
	})
	f.srv = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       5 * time.Minute,
		ErrorLog:          log.New(io.Discard, "", 0),
		ConnState: func(c net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				f.conns.Add(1)
				f.active.Add(1)
			case http.StateClosed, http.StateHijacked:
				f.active.Add(-1)
			}
		},
	}
	go func() { _ = f.srv.Serve(ln) }()
	return f, nil
}
