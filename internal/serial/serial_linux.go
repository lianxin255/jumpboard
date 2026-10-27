//go:build linux

package serial

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// List returns the serial devices, /dev/ttyS*, /dev/ttyUSB* and
// /dev/ttyACM*, sorted by name.
func List() []Port {
	matches := []string{}
	for _, pat := range []string{"/dev/ttyS*", "/dev/ttyUSB*", "/dev/ttyACM*", "/dev/ttyAMA*"} {
		if ms, _ := filepath.Glob(pat); len(ms) > 0 {
			matches = append(matches, ms...)
		}
	}
	ps := make([]Port, 0, len(matches))
	for _, m := range matches {
		ps = append(ps, Port{Name: m, Hint: filepath.Base(m)})
	}
	return normalize(ps)
}

func open(path string, rate int) (io.ReadWriteCloser, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, openErr(path, err)
	}
	if err := unix.SetNonblock(fd, false); err != nil {
		unix.Close(fd)
		return nil, openErr(path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	if f == nil {
		unix.Close(fd)
		return nil, openErr(path, os.ErrInvalid)
	}
	if err := setRaw(fd, rate); err != nil {
		f.Close()
		return nil, openErr(path, err)
	}
	return f, nil
}

// setRaw puts the port in raw, eight-bit mode at rate. Linux encodes the
// rate as a termios index, so an unsupported rate is an error here rather
// than a port left at some other speed.
func setRaw(fd, rate int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB
	t.Cflag |= unix.CS8
	if b, ok := baudIndex(rate); ok {
		t.Cflag &^= unix.CBAUD
		t.Cflag |= b
	}
	return unix.IoctlSetTermios(fd, unix.TCSETS, t)
}

func baudIndex(rate int) (uint32, bool) {
	for b, r := range map[uint32]int{
		unix.B9600: 9600, unix.B19200: 19200, unix.B38400: 38400,
		unix.B57600: 57600, unix.B115200: 115200, unix.B230400: 230400,
	} {
		if r == rate {
			return b, true
		}
	}
	return 0, false
}
