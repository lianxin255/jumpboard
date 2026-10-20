package engine

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TCP forwarder: one listener per mapping, a two-way copy per connection.

// forwarder is a running rule of any kind. Its Close must be idempotent.
type forwarder interface {
	Close() error
	// key says what the rule is bound to, so that a reapply can tell an
	// unchanged rule from a changed one.
	key() ruleKey
	// stats reports the counters the UI shows.
	stats() (connections int64, active int)
}

// ruleKey identifies what a listener is bound to and what it does, so that
// reapplying a spec keeps unchanged listeners open instead of bouncing
// them (which would drop live connections and briefly free a port).
type ruleKey struct {
	proto    model.Protocol
	listen   string
	upstream string
}

// tcpForwarder accepts connections on a port and pipes them to an upstream.
type tcpForwarder struct {
	ln       net.Listener
	upstream string
	idle     time.Duration
	conns    atomic.Int64
	active   atomic.Int32
	closing  atomic.Bool
	wg       sync.WaitGroup
}

func newTCPForwarder(listenAddr, upstream string, idle time.Duration) (*tcpForwarder, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	f := &tcpForwarder{ln: ln, upstream: upstream, idle: idle}
	f.wg.Add(1)
	go f.serve()
	return f, nil
}

func (f *tcpForwarder) key() ruleKey {
	return ruleKey{proto: model.TCP, listen: f.ln.Addr().String(), upstream: f.upstream}
}

func (f *tcpForwarder) stats() (int64, int) {
	return f.conns.Load(), int(f.active.Load())
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
		f.conns.Add(1)
		f.active.Add(1)
		go func() {
			defer f.active.Add(-1)
			pipeTCP(client, f.upstream, f.idle)
		}()
	}
}

func (f *tcpForwarder) Close() error {
	if f.closing.Swap(true) {
		return nil
	}
	err := f.ln.Close()
	f.wg.Wait()
	return err
}

// pipeTCP connects a client to the upstream and copies both ways until
// either side closes.
func pipeTCP(client net.Conn, upstream string, idle time.Duration) {
	defer client.Close()
	server, err := net.DialTimeout("tcp", upstream, 10*time.Second)
	if err != nil {
		return
	}
	defer server.Close()

	if idle > 0 {
		_ = client.SetDeadline(time.Now().Add(idle))
		_ = server.SetDeadline(time.Now().Add(idle))
	}

	done := make(chan struct{}, 2)
	copyHalf := func(dst, src net.Conn) {
		buf := make([]byte, 32<<10)
		for {
			if idle > 0 {
				_ = src.SetReadDeadline(time.Now().Add(idle))
			}
			n, err := src.Read(buf)
			if n > 0 {
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
	go copyHalf(server, client)
	go copyHalf(client, server)
	<-done
	<-done
}

// listenAddr turns a port into the address a rule listens on.
func listenAddr(port int) string { return fmt.Sprintf(":%d", port) }
