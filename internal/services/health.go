// Package services holds the app's background work: health checks, port
// probing, log tailing, templates and data transfer.
package services

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// checkTimeout bounds one health check, so a hung upstream cannot stall the
// loop.
const checkTimeout = 3 * time.Second

// CheckTCP reports whether a TCP connection to the upstream succeeds.
func CheckTCP(host string, port int) model.HealthStatus {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, itoa(port)), checkTimeout)
	if err != nil {
		return model.HealthUnhealthy
	}
	conn.Close()
	return model.HealthHealthy
}

// CheckHTTP requests a path from the upstream and compares the status.
//
// TLS certificates are not verified: the point is whether the service
// answers, and an internal service usually has a self-signed one.
func CheckHTTP(host string, port int, path string, expected int, useTLS bool) model.HealthStatus {
	if path == "" {
		path = "/"
	}
	if expected == 0 {
		expected = http.StatusOK
	}
	client := &http.Client{
		Timeout: checkTimeout,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	url := fmt.Sprintf("http://%s%s", net.JoinHostPort(host, itoa(port)), path)
	if useTLS {
		url = fmt.Sprintf("https://%s%s", net.JoinHostPort(host, itoa(port)), path)
	}
	resp, err := client.Get(url)
	if err != nil {
		return model.HealthUnhealthy
	}
	defer resp.Body.Close()
	if resp.StatusCode == expected {
		return model.HealthHealthy
	}
	return model.HealthUnhealthy
}

// CheckMapping probes one mapping's upstream.
func CheckMapping(m model.Mapping) model.HealthStatus {
	if m.Protocol.IsStream() {
		// UDP has no connection to open: a TCP probe tells whether the
		// host is up at all, which is what the old build did too.
		return CheckTCP(m.UpstreamHost, m.UpstreamPort)
	}
	return CheckHTTP(m.UpstreamHost, m.UpstreamPort,
		m.HealthCheckTargetPath, m.HealthCheckExpectedCode, m.Protocol == model.HTTPS)
}

// HealthChange is one mapping whose health status moved, with where it
// came from: a rule that was fine and stopped answering is news, while
// one that was never checked turning unhealthy is the first look.
type HealthChange struct {
	Mapping model.Mapping
	From    model.HealthStatus
	To      model.HealthStatus
}

// Checker runs the health checks on a timer and reports each result.
type Checker struct {
	store *store.Store

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
	// done closes when Stop runs, so a goroutine waiting on the interval
	// wakes up and leaves instead of hanging on a stopped timer.
	done chan struct{}
	// OnResult is called from the loop's goroutine after each pass, with
	// the mappings that changed status.
	OnResult func(changed []HealthChange)
	// Interval is how often a pass runs; 30 seconds when zero.
	Interval time.Duration
}

// NewChecker returns a checker reading its mappings from s.
func NewChecker(s *store.Store) *Checker {
	return &Checker{store: s, Interval: 30 * time.Second}
}

// RunOnce checks every mapping that has checks turned on.
func (c *Checker) RunOnce() []HealthChange {
	mappings, err := c.store.ListEnabledMappings()
	if err != nil {
		return nil
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		changed []HealthChange
	)
	now := time.Now().UnixMilli()
	for _, m := range mappings {
		if !m.HealthCheckEnabled {
			continue
		}
		wg.Add(1)
		go func(m model.Mapping) {
			defer wg.Done()
			from := m.HealthStatus
			status := CheckMapping(m)
			_ = c.store.SetMappingHealth(m.ID, status, now)
			if status != from {
				m.HealthStatus, m.LastCheckedAt = status, now
				mu.Lock()
				changed = append(changed, HealthChange{Mapping: m, From: from, To: status})
				mu.Unlock()
			}
		}(m)
	}
	wg.Wait()
	return changed
}

// Start begins the loop. Calling it twice restarts the timer.
func (c *Checker) Start() {
	c.mu.Lock()
	// A loop already running must end first: its goroutine waits on the
	// done channel this call replaces, and closing that channel wakes it.
	c.stopLocked()
	c.stopped = false
	interval := c.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	done := make(chan struct{})
	c.done = done
	c.mu.Unlock()

	go func() {
		for {
			changed := c.RunOnce()
			if len(changed) > 0 && c.OnResult != nil {
				c.OnResult(changed)
			}
			c.mu.Lock()
			if c.stopped {
				c.mu.Unlock()
				return
			}
			c.timer = time.NewTimer(interval)
			t := c.timer
			c.mu.Unlock()
			// Wait for the interval or for Stop: stopping the timer alone
			// leaves this goroutine blocked on a channel that never fires,
			// so every Start/Stop pair used to strand one.
			select {
			case <-t.C:
			case <-done:
				return
			}
		}
	}()
}

// Stop ends the loop.
func (c *Checker) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

// stopLocked signals the loop to leave. Idempotent: the second call sees
// done already nil.
func (c *Checker) stopLocked() {
	c.stopped = true
	if c.timer != nil {
		c.timer.Stop()
	}
	if c.done != nil {
		close(c.done)
		c.done = nil
	}
}
