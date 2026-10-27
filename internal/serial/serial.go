// Package serial opens serial ports, for the terminal page's 串口 sessions.
// It knows only how to open a device at a baud rate and put it in raw,
// eight-bit mode — nothing about protocols, credentials or remote hosts,
// which keeps it out of the SSH surface entirely. It is two files of
// platform code behind one interface: termios on the Unixes, a DCB on
// Windows, where a serial port is a COMM device opened with CreateFile.
package serial

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Port is one serial device the machine offers.
type Port struct {
	// Name is the device path or the Windows COMM name, what Open takes.
	Name string
	// Hint is a human note — the device's description on Windows, the
	// last path component elsewhere, or "" when the system names nothing.
	Hint string
}

// Baud is a rate the UI offers.
type Baud int

// Rates the UI offers. A port accepts what the hardware does; the list
// stays the rates a jump box is likely to answer.
const (
	Baud9600   Baud = 9600
	Baud19200  Baud = 19200
	Baud38400  Baud = 38400
	Baud57600  Baud = 57600
	Baud115200 Baud = 115200
	Baud230400 Baud = 230400
)

// AllRates is the list of rates, in ascending order, for a picker.
func AllRates() []Baud {
	return []Baud{Baud9600, Baud19200, Baud38400, Baud57600, Baud115200, Baud230400}
}

// Open opens the port at rate and puts it in raw, eight-bit mode: no
// echo, no canonical line editing, no translations. The returned
// io.ReadWriteCloser is what the terminal's Conn takes.
func Open(path string, rate Baud) (io.ReadWriteCloser, error) {
	return open(path, int(rate))
}

// openErr wraps a platform error with the port's name, so the pane says
// which device refused it.
func openErr(path string, err error) error {
	return fmt.Errorf("%s: %w", path, err)
}

// normalize sorts the ports by name, so the picker's order is stable
// across calls, and drops the ones the platform listed without a usable
// name.
func normalize(ps []Port) []Port {
	out := ps[:0]
	for _, p := range ps {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
