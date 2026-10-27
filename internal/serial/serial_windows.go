//go:build windows

package serial

import (
	"io"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DCB.Flags holds the port's mode as bit flags; only Fbinary is ours to
// force. The others (parity, flow control) keep what the driver reported,
// which is what the cable and the device actually negotiate.
const (
	fBinary = 0x00000001
	// maxDword is the interval timeout that makes a raw serial read wait
	// for the first byte rather than returning empty at once.
	maxDword = 0xffffffff
)

// List returns the serial devices Windows reports, the COMM names —
// COM3, COM5, … — with their device descriptions as the hint. A device
// is listed when its name appears in the registry's serial-enum tree; the
// description needs a working WMI, which may not be, so it is optional.
func List() []Port {
	names, descs := commNames()
	ps := make([]Port, 0, len(names))
	for _, n := range names {
		ps = append(ps, Port{Name: n, Hint: descs[n]})
	}
	return normalize(ps)
}

func open(name string, rate int) (io.ReadWriteCloser, error) {
	if name == "" {
		return nil, openErr(name, syscall.EINVAL)
	}
	path, err := windows.UTF16PtrFromString(`\\.\` + name)
	if err != nil {
		return nil, openErr(name, err)
	}
	h, err := windows.CreateFile(path,
		windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, openErr(name, err)
	}
	if err := setMode(h, rate); err != nil {
		windows.CloseHandle(h)
		return nil, openErr(name, err)
	}
	return &commFile{h: h, name: name}, nil
}

// setMode configures the port: the rate, eight data bits, no parity, one
// stop bit, binary mode, and the timeouts that make a read wait for a
// byte. The driver's own flow-control flags are left alone.
func setMode(h windows.Handle, rate int) error {
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(h, &dcb); err != nil {
		return err
	}
	dcb.BaudRate = uint32(rate)
	dcb.ByteSize = 8
	dcb.Parity = 0 // no parity
	dcb.StopBits = 0
	dcb.Flags = dcb.Flags | fBinary
	if err := windows.SetCommState(h, &dcb); err != nil {
		return err
	}
	to := windows.CommTimeouts{
		ReadIntervalTimeout:         maxDword,
		ReadTotalTimeoutMultiplier:  0,
		ReadTotalTimeoutConstant:    0,
		WriteTotalTimeoutMultiplier: 0,
		WriteTotalTimeoutConstant:   0,
	}
	return windows.SetCommTimeouts(h, &to)
}

// commFile is the port as an io.ReadWriteCloser, a handle with a mutex so
// a write from the terminal and one from elsewhere do not interleave
// bytes.
type commFile struct {
	h    windows.Handle
	name string
	mu   sync.Mutex
}

func (f *commFile) Read(p []byte) (int, error) {
	for {
		f.mu.Lock()
		var n uint32
		err := windows.ReadFile(f.h, p, &n, nil)
		f.mu.Unlock()
		if n > 0 || err != nil {
			return int(n), err
		}
		// A driver that treats the interval timeout as "nothing there, now"
		// returns empty with no error; a read that busy-loops would spin a
		// goroutine, so the empty read waits a moment before it asks again.
		time.Sleep(20 * time.Millisecond)
	}
}

func (f *commFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n uint32
	if err := windows.WriteFile(f.h, p, &n, nil); err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (f *commFile) Close() error {
	return windows.CloseHandle(f.h)
}
