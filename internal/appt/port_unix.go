//go:build !windows

package appt

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// portFree reports whether the port can be bound on all interfaces.
func portFree(port int) bool {
	l, err := net.Listen("tcp", ":"+itoa(port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// portFreeUDP reports whether the UDP port can be bound on all interfaces.
func portFreeUDP(port int) bool {
	c, err := net.ListenPacket("udp", ":"+itoa(port))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// whoHolds names the processes holding a port, best effort: whatever lsof
// can say on this machine, nothing when it cannot — absent, slow or silent.
// The caller has already proved the port taken by failing to bind it; this
// is the decoration that turns "cannot bind" into "who is sitting on it".
func whoHolds(port int, proto string) []procEntry {
	args := []string{"-nP", "-Fpc", "-i" + proto + ":" + itoa(port)}
	if proto == "TCP" {
		// A listening socket is what a forwarder collides with; the
		// port's other sockets belong to whoever is listening.
		args = append(args, "-sTCP:LISTEN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, err := runTool(ctx, "lsof", args)
	if err != nil {
		return nil
	}
	return parseLSOF(out)
}

// parseLSOF pairs lsof -F's process records into one entry per process.
// The records repeat once per file descriptor, p before each set, so the
// pairs are collected in first-seen order and the repeats collapse; a c
// with no p in front of it (there is none in practice) is skipped.
func parseLSOF(out string) []procEntry {
	var (
		seen  = map[string]string{}
		order []string
		pid   string
	)
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		val := line[1:]
		switch line[0] {
		case 'p':
			pid = val
		case 'c':
			if pid == "" {
				continue
			}
			if _, ok := seen[pid]; !ok {
				order = append(order, pid)
			}
			seen[pid] = val
		}
	}
	entries := make([]procEntry, 0, len(order))
	for _, pid := range order {
		entries = append(entries, procEntry{PID: pid, Name: seen[pid]})
	}
	return entries
}

// keep model imported for bindable's signature on every build.
var _ = model.TCP
