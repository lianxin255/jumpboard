//go:build !windows

package nginx

import (
	"os"
	"os/exec"
	"syscall"
)

// hideConsole does nothing on Unix, which has no console window to hide.
func hideConsole(cmd *exec.Cmd) {}

// killTree ends a process and the workers it started.
//
// SIGKILL rather than SIGTERM: the caller has already asked nginx to quit
// politely and waited; anything still alive is stuck.
func killTree(pid int) {
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// pidAlive reports whether a process with that ID exists.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix FindProcess always succeeds; signal 0 only probes.
	return p.Signal(syscall.Signal(0)) == nil
}
