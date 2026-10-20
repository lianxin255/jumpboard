//go:build windows

package nginx

import "os"

// findProcess opens a handle to a process, which fails when it is gone.
func findProcess(pid int) (*os.Process, error) { return os.FindProcess(pid) }
