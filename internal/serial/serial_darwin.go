//go:build darwin

package serial

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// List returns the dial-out serial devices, /dev/cu.* — the ports meant
// for connecting out, which a terminal session wants — sorted by name.
func List() []Port {
	matches, _ := filepath.Glob("/dev/cu.*")
	ps := make([]Port, 0, len(matches))
	for _, m := range matches {
		ps = append(ps, Port{Name: m, Hint: filepath.Base(m)})
	}
	return normalize(ps)
}

func open(path string, rate int) (io.ReadWriteCloser, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, openErr(path, err)
	}
	// The descriptor is opened non-blocking so an absent device fails at
	// once; the file it becomes must block, or a read returns immediately
	// with nothing and the terminal spins.
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

// setRaw puts the port in raw, eight-bit mode at rate. Darwin keeps the
// rate as a number in Ispeed/Ospeed, so a rate the hardware lacks fails
// at the ioctl rather than being silently approximated.
func setRaw(fd, rate int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB
	t.Cflag |= unix.CS8
	t.Ispeed, t.Ospeed = uint64(rate), uint64(rate)
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}
