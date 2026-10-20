//go:build !windows

package appt

import "net"

// portFree reports whether the port can be bound on all interfaces.
func portFree(port int) bool {
	l, err := net.Listen("tcp", ":"+itoa(port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}
