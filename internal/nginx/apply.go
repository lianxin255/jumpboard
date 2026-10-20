package nginx

import (
	"errors"
	"fmt"
	"time"
)

// ApplyResult is the outcome of writing and applying a configuration.
type ApplyResult struct {
	OK bool
	// Stage says where it failed: writing the file, nginx -t, or the
	// reload. The caller logs a user error and a runtime error apart.
	Stage  string
	Output string
}

// Apply writes a configuration and makes nginx read it, rolling the file
// back when a step fails.
//
// The sequence is: save what is on disk, write the new file, run nginx -t,
// reload, and on any failure put the old file back. Restoring matters most
// after a failed reload: the old workers keep serving the old
// configuration, so forwarding is fine until the next restart, which would
// then load the broken file and take everything down.
//
// skipReload is for an engine that is not running: the file is written and
// checked, and nginx loads it when it starts.
func (e *Engine) Apply(conf string, skipReload bool) ApplyResult {
	e.confMu.Lock()
	defer e.confMu.Unlock()

	old, err := e.ReadConf()
	if err != nil {
		return ApplyResult{Stage: "io", Output: "读取现有配置失败: " + err.Error()}
	}
	if err := e.WriteConf(conf); err != nil {
		return ApplyResult{Stage: "io", Output: "写入配置失败: " + err.Error()}
	}

	ok, out := e.Test()
	if !ok {
		e.restore(old)
		return ApplyResult{Stage: "test", Output: out}
	}
	if skipReload {
		return ApplyResult{OK: true, Output: out}
	}
	if err := e.Reload(); err != nil {
		e.restore(old)
		return ApplyResult{Stage: "reload", Output: err.Error()}
	}
	return ApplyResult{OK: true, Output: out}
}

// restore puts back the configuration Apply found on disk.
func (e *Engine) restore(old string) {
	if old == "" {
		// There was no configuration: leaving the broken one would make
		// the next start fail, so remove it and let the caller rebuild.
		_ = e.WriteConf(minimalConf())
		return
	}
	_ = e.WriteConf(old)
}

// minimalConf is a configuration nginx accepts with no mappings, used when
// there is nothing to roll back to.
func minimalConf() string {
	return `# 由 nginx-jump-gui 生成：无映射时的最小配置
worker_processes 1;
error_log logs/error.log warn;
pid temp/nginx.pid;
events {
  worker_connections 1024;
}
`
}

// StartOrReload applies a configuration and leaves nginx running with it:
// a stopped engine is started, a running one reloaded.
func (e *Engine) StartOrReload(conf string) ApplyResult {
	skipReload := e.Status() != "running"
	res := e.Apply(conf, skipReload)
	if !res.OK {
		return res
	}
	if skipReload {
		if err := e.Start(); err != nil {
			return ApplyResult{Stage: "start", Output: err.Error()}
		}
	}
	return res
}

// WaitListening waits until a TCP port on the loopback interface accepts
// connections, which is how the tests know nginx finished starting.
func WaitListening(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if PortOpen(port) {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("端口 %d 在 %v 内没有开始监听", port, timeout)
}

// WaitClosed waits until nothing listens on a port any more.
func WaitClosed(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !PortOpen(port) {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("端口 %d 在 %v 后仍在监听", port, timeout)
}

// ErrNotRunning is returned by operations that need a running engine.
var ErrNotRunning = errors.New("nginx 未运行")
