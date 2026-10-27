//go:build windows

package appt

import (
	"bufio"
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

// whoHolds names the processes holding a port on Windows: netstat -ano
// finds the PID of the listening socket, and tasklist finds its image
// name. Best-effort: missing tools or permissions fall back to the PID
// alone or an empty list, and the caller degrades to "port taken".
func whoHolds(port int, proto string) []procEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, err := runTool(ctx, "netstat", []string{"-ano"})
	if err != nil {
		return nil
	}
	pids := parseNetstatANO(out, port, proto)
	if len(pids) == 0 {
		return nil
	}
	names := tasklistNames(pids)
	entries := make([]procEntry, 0, len(pids))
	for _, pid := range pids {
		entries = append(entries, procEntry{PID: pid, Name: names[pid]})
	}
	return entries
}

// parseNetstatANO extracts the PIDs listening on port for the protocol.
// Lines are "TCP  0.0.0.0:8080  0.0.0.0:0  LISTENING  1234" or
// "UDP  0.0.0.0:53  *:*  1234".
func parseNetstatANO(out string, port int, proto string) []string {
	targetPort := ":" + itoa(port)
	seen := map[string]bool{}
	var pids []string
	protoUpper := strings.ToUpper(proto)

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		if !strings.EqualFold(fields[0], protoUpper) {
			continue
		}
		localAddr := fields[1]
		if !strings.HasSuffix(localAddr, targetPort) {
			continue
		}
		// For TCP, keep only LISTENING rows.
		var pid string
		if protoUpper == "TCP" {
			if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
				continue
			}
			pid = fields[4]
		} else {
			pid = fields[len(fields)-1]
		}
		if pid == "" || pid == "0" || seen[pid] {
			continue
		}
		seen[pid] = true
		pids = append(pids, pid)
	}
	return pids
}

// tasklistNames resolves PIDs to process image names using tasklist /FO CSV.
func tasklistNames(pids []string) map[string]string {
	names := map[string]string{}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, err := runTool(ctx, "tasklist", []string{"/FO", "CSV", "/NH"})
	if err != nil {
		return names
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		img := strings.Trim(parts[0], `" `)
		pid := strings.Trim(parts[1], `" `)
		for _, want := range pids {
			if pid == want {
				names[pid] = img
			}
		}
	}
	return names
}

var _ = model.TCP
