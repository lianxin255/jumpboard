//go:build windows

package nginx

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps nginx's console from flashing up.
const createNoWindow = 0x08000000

// hideConsole starts the process without a console window.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// killTree ends a process and the workers it started.
//
// taskkill /T is the only reliable way: killing the master with
// Process.Kill leaves the workers running as orphans (nginx trac #2394),
// and Go kills only the process it started.
func killTree(pid int) {
	if pid <= 0 {
		return
	}
	cmd := exec.Command("taskkill", "/PID", itoa(pid), "/T", "/F")
	hideConsole(cmd)
	_ = cmd.Run()
}

// pidAlive reports whether a process with that ID exists.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// On Windows os.FindProcess opens a handle that fails for a PID that
	// no longer exists.
	p, err := findProcess(pid)
	if err != nil {
		return false
	}
	p.Release()
	return true
}
