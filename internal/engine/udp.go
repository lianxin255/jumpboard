package engine

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// UDP forwarder: one socket per mapping, one upstream socket per client
// address, expired after the rule's idle timeout.

// udpForwarder forwards datagrams.
//
// UDP has no connection, so the forwarder keeps a session per client
// address: the first datagram from an address opens a socket towards the
// upstream, and replies are sent back to that address. A session closes
// after idle.
type udpForwarder struct {
	conn *net.UDPConn
	// upstream is mutable, so retargeting a rule keeps its port.
	upstream atomic.Pointer[string]
	idle     time.Duration
	// whitelist limits which clients may send, the same filter the HTTP
	// and TCP paths apply.
	whitelist []*net.IPNet

	mu       sync.Mutex
	sessions map[string]*udpSession
	closing  atomic.Bool
	wg       sync.WaitGroup
	conns    atomic.Int64
	active   atomic.Int32
}

type udpSession struct {
	clientAddr *net.UDPAddr
	upstream   *net.UDPConn
	lastSeen   atomic.Int64
}

func newUDPForwarder(listenAddr, upstream string, idle time.Duration, whitelist []*net.IPNet) (*udpForwarder, error) {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	f := &udpForwarder{
		conn:      conn,
		idle:      idle,
		whitelist: whitelist,
		sessions:  map[string]*udpSession{},
	}
	f.upstream.Store(&upstream)
	f.wg.Add(2)
	go f.readClients()
	go f.reapSessions()
	return f, nil
}

// setUpstream points the rule at a new backend. Existing sessions are
// dropped: their sockets lead to the old one, and a datagram protocol has
// no way to move them.
func (f *udpForwarder) setUpstream(upstream string) {
	f.upstream.Store(&upstream)
	f.mu.Lock()
	var stale []string
	for key := range f.sessions {
		stale = append(stale, key)
	}
	f.mu.Unlock()
	for _, key := range stale {
		f.dropSessionByKey(key)
	}
}

func (f *udpForwarder) target() string { return *f.upstream.Load() }

func (f *udpForwarder) key() ruleKey {
	return ruleKey{proto: model.UDP, listen: f.conn.LocalAddr().String(), upstream: f.target()}
}

func (f *udpForwarder) stats() (int64, int) {
	return f.conns.Load(), int(f.active.Load())
}

func (f *udpForwarder) readClients() {
	defer f.wg.Done()
	buf := make([]byte, 64<<10)
	for {
		n, clientAddr, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			if f.closing.Load() {
				return
			}
			continue
		}
		if !allowed(f.whitelist, clientAddr.String()) {
			continue
		}
		s := f.session(clientAddr)
		if s == nil {
			// The upstream name does not resolve, or its socket cannot be
			// opened: drop this datagram rather than write to a nil socket,
			// which used to panic the whole process from a goroutine.
			continue
		}
		s.lastSeen.Store(time.Now().UnixNano())
		if _, err := s.upstream.Write(buf[:n]); err != nil {
			f.dropSession(clientAddr)
		}
	}
}

// session returns the session of a client, creating it on first use.
//
// A nil return means the upstream could not be reached; the caller drops
// the datagram. The failure is not cached, so a name that starts resolving
// works on the next packet.
func (f *udpForwarder) session(client *net.UDPAddr) *udpSession {
	key := client.String()
	f.mu.Lock()
	if s, ok := f.sessions[key]; ok {
		f.mu.Unlock()
		return s
	}
	upstreamAddr, err := net.ResolveUDPAddr("udp", f.target())
	if err != nil {
		f.mu.Unlock()
		return nil
	}
	up, err := net.DialUDP("udp", nil, upstreamAddr)
	if err != nil {
		f.mu.Unlock()
		return nil
	}
	s := &udpSession{clientAddr: client, upstream: up}
	s.lastSeen.Store(time.Now().UnixNano())
	f.sessions[key] = s
	f.conns.Add(1)
	f.active.Add(1)
	f.wg.Add(1)
	f.mu.Unlock()

	go f.readUpstream(s, key)
	return s
}

// readUpstream sends the upstream's replies back to the client.
func (f *udpForwarder) readUpstream(s *udpSession, key string) {
	defer f.wg.Done()
	buf := make([]byte, 64<<10)
	for {
		if s.upstream == nil {
			return
		}
		if f.idle > 0 {
			_ = s.upstream.SetReadDeadline(time.Now().Add(f.idle))
		}
		n, err := s.upstream.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				f.dropSessionByKey(key)
				return
			}
			f.dropSessionByKey(key)
			return
		}
		if _, err := f.conn.WriteToUDP(buf[:n], s.clientAddr); err != nil {
			f.dropSessionByKey(key)
			return
		}
	}
}

// reapSessions closes the sessions nobody used for a while.
func (f *udpForwarder) reapSessions() {
	defer f.wg.Done()
	interval := f.idle / 2
	if interval <= 0 || interval > 30*time.Second {
		interval = 30 * time.Second
	}
	for {
		time.Sleep(interval)
		if f.closing.Load() {
			return
		}
		cutoff := time.Now().Add(-f.idle).UnixNano()
		f.mu.Lock()
		var stale []string
		for key, s := range f.sessions {
			if s.lastSeen.Load() < cutoff {
				stale = append(stale, key)
			}
		}
		f.mu.Unlock()
		for _, key := range stale {
			f.dropSessionByKey(key)
		}
	}
}

func (f *udpForwarder) dropSession(addr *net.UDPAddr) { f.dropSessionByKey(addr.String()) }

func (f *udpForwarder) dropSessionByKey(key string) {
	f.mu.Lock()
	s, ok := f.sessions[key]
	if ok {
		delete(f.sessions, key)
	}
	f.mu.Unlock()
	if ok {
		if s.upstream != nil {
			_ = s.upstream.Close()
		}
		f.active.Add(-1)
	}
}

func (f *udpForwarder) Close() error {
	if f.closing.Swap(true) {
		return nil
	}
	err := f.conn.Close()
	f.mu.Lock()
	for key, s := range f.sessions {
		if s.upstream != nil {
			_ = s.upstream.Close()
		}
		delete(f.sessions, key)
	}
	f.mu.Unlock()
	return err
}
