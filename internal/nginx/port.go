package nginx

import "net"

// PortOpen reports whether something accepts TCP connections on the port.
func PortOpen(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 200*timeMillis)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// PortFree reports whether the port can be bound, which is what the app
// checks before writing a configuration that would fail to start.
func PortFree(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	l, err := net.Listen("tcp", ":"+itoa(port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}
