package engine

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TCP forwarder: one listener per mapping, a two-way copy per connection.

// forwarder is a running rule of any kind. Its Close must be idempotent,
// must return promptly, and must leave nothing writing to a shared
// resource (the access log) once it has: Stop closes the log after the
// forwarders.
type forwarder interface {
	Close() error
	// key says what the rule is bound to, so that a reapply can tell an
	// unchanged rule from a changed one.
	key() ruleKey
	// stats reports the counters the UI shows. mappingID names the rule
	// asked about: an HTTP port serves several rules and counts each one's
	// bytes apart, while a stream listener serves one rule and ignores the
	// name.
	stats(mappingID string) ruleStats
}

// ruleStats is what a forwarder reports about the rule it serves.
type ruleStats struct {
	connections int64
	active      int
	// bytesIn and bytesOut are the body or payload bytes the rule moved,
	// clients in and back out again.
	bytesIn  int64
	bytesOut int64
}

// ruleKey identifies what a listener is bound to and what it does, so that
// reapplying a spec keeps unchanged listeners open instead of bouncing
// them (which would drop live connections and briefly free a port).
//
// The upstream slot doubles as the whitelist fingerprint of the redirect
// listener: a redirect that keeps running after the list changes leaks the
// site to a client the user just removed.
type ruleKey struct {
	proto    model.Protocol
	listen   string
	upstream string
}

// tcpForwarder accepts connections on a port and pipes them to an upstream.
//
// The upstream is mutable so that pointing a rule at a different backend
// changes it in place: closing and reopening the listener would drop every
// live connection and, while the old one still holds the port, fail to bind
// at all.
type tcpForwarder struct {
	ln net.Listener
	// upstream is read per connection and swapped by setUpstream.
	upstream atomic.Pointer[string]
	idle     time.Duration
	conns    atomic.Int64
	active   atomic.Int32
	// bytesIn and bytesOut are the bytes the rule moved, accumulated by
	// every pipe it served. They live on the forwarder so a retarget —
	// which keeps the listener — keeps the rule's totals, like conns.
	bytesIn  atomic.Int64
	bytesOut atomic.Int64
	closing  atomic.Bool
	wg       sync.WaitGroup
	// access logs each connection, as nginx's stream access_log does.
	access *accessLog
	// whitelist limits which clients may connect, the same filter the HTTP
	// path applies. Without it a TCP rule — SSH, RDP, MySQL — would answer
	// every address even when the whitelist is on, which is the opposite of
	// what the setting promises.
	whitelist []*net.IPNet

	// mu guards live: the pipes running now, closable from Close.
	mu   sync.Mutex
	live map[*tcpPipe]struct{}
}

func newTCPForwarder(listenAddr, upstream string, idle time.Duration, access *accessLog, whitelist []*net.IPNet) (*tcpForwarder, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	f := &tcpForwarder{ln: ln, idle: idle, access: access, whitelist: whitelist, live: map[*tcpPipe]struct{}{}}
	f.upstream.Store(&upstream)
	f.wg.Add(1)
	go f.serve()
	return f, nil
}

// setUpstream points the rule at a new backend without touching the
// listener, so the port stays bound and live connections keep running.
func (f *tcpForwarder) setUpstream(upstream string) { f.upstream.Store(&upstream) }

func (f *tcpForwarder) target() string { return *f.upstream.Load() }

func (f *tcpForwarder) key() ruleKey {
	return ruleKey{proto: model.TCP, listen: f.ln.Addr().String(), upstream: f.target()}
}

func (f *tcpForwarder) stats(string) ruleStats {
	return ruleStats{
		connections: f.conns.Load(),
		active:      int(f.active.Load()),
		bytesIn:     f.bytesIn.Load(),
		bytesOut:    f.bytesOut.Load(),
	}
}

func (f *tcpForwarder) serve() {
	defer f.wg.Done()
	for {
		client, err := f.ln.Accept()
		if err != nil {
			if f.closing.Load() {
				return
			}
			// A transient accept error (too many files) should not kill
			// the rule; a closing listener must.
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if f.closing.Load() {
				return
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		// The address filter applies to a plain port forward too: a
		// whitelist that skipped TCP would leave the sensitive rules
		// open while the web ones are closed. A refused client is not
		// a served one, so it never reaches the counters.
		if !allowed(f.whitelist, client.RemoteAddr().String()) {
			_ = client.Close()
			continue
		}
		p := &tcpPipe{client: client, upstream: f.target(), idle: f.idle,
			in: &f.bytesIn, out: &f.bytesOut}
		f.mu.Lock()
		if f.closing.Load() {
			f.mu.Unlock()
			_ = client.Close()
			continue
		}
		f.live[p] = struct{}{}
		f.mu.Unlock()
		f.conns.Add(1)
		f.active.Add(1)
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			defer f.active.Add(-1)
			start := time.Now()
			p.run()
			f.mu.Lock()
			delete(f.live, p)
			f.mu.Unlock()
			if f.access != nil {
				f.access.writeStream(client.RemoteAddr().String(), p.upstream, time.Since(start))
			}
		}()
	}
}

func (f *tcpForwarder) Close() error {
	if f.closing.Swap(true) {
		return nil
	}
	err := f.ln.Close()
	// Cut the live pipes before waiting: a forwarded SSH session has no
	// reason to end on its own, and waiting for it would hang the app's
	// quit.
	f.mu.Lock()
	for p := range f.live {
		p.Close()
	}
	f.mu.Unlock()
	f.wg.Wait()
	return err
}

// tcpPipe owns one forwarded connection pair.
//
// The upstream end appears mid-run, when the dial completes, and Close may
// come first: both ends are tracked so that nothing is left dialing or
// copying after the forwarder is closed.
type tcpPipe struct {
	client   net.Conn
	upstream string
	idle     time.Duration
	// in and out are the rule's counters: what this pipe reads from the
	// client counts as traffic in, what it reads from the upstream as
	// traffic out.
	in  *atomic.Int64
	out *atomic.Int64

	mu     sync.Mutex
	server net.Conn
	closed bool
}

func (p *tcpPipe) Close() {
	p.mu.Lock()
	p.closed = true
	server := p.server
	p.mu.Unlock()
	_ = p.client.Close()
	if server != nil {
		_ = server.Close()
	}
}

// run connects the client to the upstream and copies both ways until
// either side closes.
func (p *tcpPipe) run() {
	defer p.client.Close()
	server, err := net.DialTimeout("tcp", p.upstream, 10*time.Second)
	if err != nil {
		return
	}
	p.mu.Lock()
	abandoned := p.closed
	if !abandoned {
		p.server = server
	}
	p.mu.Unlock()
	if abandoned {
		// The forwarder closed while the dial was in flight.
		_ = server.Close()
		return
	}
	defer server.Close()

	done := make(chan struct{}, 2)
	copyHalf := func(dst, src net.Conn, counter *atomic.Int64) {
		buf := make([]byte, 32<<10)
		for {
			// Only the read side carries the idle deadline, renewed each
			// round. A one-shot deadline set when the connection opened
			// would cut off an SSH or database session that is alive but
			// quiet — the deadline should expire on silence, not on age.
			if p.idle > 0 {
				_ = src.SetReadDeadline(time.Now().Add(p.idle))
			}
			n, err := src.Read(buf)
			if n > 0 {
				counter.Add(int64(n))
				if _, werr := dst.Write(buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		// Half-close so that a client which shut down its sending side
		// still receives the upstream's answer.
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyHalf(server, p.client, p.in)
	go copyHalf(p.client, server, p.out)
	<-done
	<-done
}

// listenAddr turns a port into the address a rule listens on.
func listenAddr(port int) string { return fmt.Sprintf(":%d", port) }
