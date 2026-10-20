package nginx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Engine runs the bundled nginx as a child process.
type Engine struct {
	bin      string
	prefix   string
	confPath string

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	lastErr string
	// confMu serializes the write-test-reload sequence, so that two
	// applies in flight cannot interleave into a half-written file.
	confMu sync.Mutex
}

// NewEngine returns an engine for the nginx at bin, with prefix as its -p
// directory.
func NewEngine(bin, prefix string) *Engine {
	return &Engine{
		bin:      bin,
		prefix:   prefix,
		confPath: filepath.Join(prefix, "conf", "nginx.conf"),
	}
}

// ConfPath returns the file the engine reads at startup.
func (e *Engine) ConfPath() string { return e.confPath }

// prepareDir creates the directories nginx writes into.
func (e *Engine) prepareDir() error {
	for _, d := range []string{"conf", "logs", "temp"} {
		if err := os.MkdirAll(filepath.Join(e.prefix, d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// WriteConf writes the configuration to disk, without touching nginx.
func (e *Engine) WriteConf(conf string) error {
	if err := e.prepareDir(); err != nil {
		return err
	}
	return os.WriteFile(e.confPath, []byte(conf), 0o644)
}

// ReadConf returns the configuration on disk, or "" when there is none.
func (e *Engine) ReadConf() (string, error) {
	b, err := os.ReadFile(e.confPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// Test runs nginx -t on the configuration on disk.
func (e *Engine) Test() (bool, string) {
	if err := e.prepareDir(); err != nil {
		return false, err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.bin, "-t", "-p", e.prefix, "-c", "conf/nginx.conf")
	cmd.Dir = e.prefix
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return false, "nginx -t 超时"
		}
		return false, strings.TrimSpace(string(out))
	}
	return true, strings.TrimSpace(string(out))
}

// Start launches nginx in the background.
//
// A process still alive 500 ms after starting counts as started: nginx
// exits at once when it cannot bind a port or read its configuration, and
// its stderr says why.
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("nginx 已在运行")
	}
	e.mu.Unlock()

	if _, err := os.Stat(e.bin); err != nil {
		return fmt.Errorf("找不到内置 nginx: %w", err)
	}
	if err := e.prepareDir(); err != nil {
		return err
	}

	cmd := exec.Command(e.bin, "-p", e.prefix, "-c", "conf/nginx.conf", "-g", "daemon off;")
	cmd.Dir = e.prefix
	hideConsole(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 nginx: %w", err)
	}

	// Collect stderr for the diagnosis: it is where nginx says why it
	// refused to start.
	var (
		bufMu sync.Mutex
		buf   strings.Builder
	)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			bufMu.Lock()
			if buf.Len() < 64<<10 {
				buf.WriteString(sc.Text())
				buf.WriteByte('\n')
			}
			bufMu.Unlock()
		}
	}()
	tail := func() string {
		bufMu.Lock()
		defer bufMu.Unlock()
		return strings.TrimSpace(buf.String())
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err := <-exited:
		e.mu.Lock()
		e.lastErr = tail()
		e.mu.Unlock()
		detail := tail()
		if detail != "" {
			return fmt.Errorf("nginx 启动后立即退出: %v\n%s", err, detail)
		}
		return fmt.Errorf("nginx 启动后立即退出: %v", err)
	case <-time.After(500 * time.Millisecond):
	}

	e.mu.Lock()
	e.cmd, e.running, e.lastErr = cmd, true, ""
	e.mu.Unlock()

	go func() {
		<-exited
		e.mu.Lock()
		if e.cmd == cmd {
			e.cmd, e.running = nil, false
		}
		e.mu.Unlock()
	}()
	return nil
}

// Stop quits nginx and waits for it to exit, killing it if it does not.
func (e *Engine) Stop() error {
	e.mu.Lock()
	cmd := e.cmd
	e.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Ask nginx to quit itself, so that it removes its pid file and lets
	// the workers finish their connections.
	_, _ = e.signal("quit")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		running := e.running
		e.mu.Unlock()
		if !running {
			return nil
		}
		time.Sleep(30 * time.Millisecond)
	}
	killTree(cmd.Process.Pid)
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
	return nil
}

// Reload makes nginx read the configuration again.
//
// SIGHUP reaches the master on Unix. Windows has no signals: nginx -s
// reload finds the master through the pid file instead.
func (e *Engine) Reload() error {
	e.mu.Lock()
	running := e.running
	e.mu.Unlock()
	if !running {
		return errors.New("nginx 未运行，无法 reload")
	}
	if ok, out := e.signal("reload"); !ok && out != "" {
		return errors.New(out)
	}
	return nil
}

// signal runs nginx -s <sig>, which reads the pid file.
func (e *Engine) signal(sig string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.bin, "-p", e.prefix, "-c", "conf/nginx.conf", "-s", sig)
	cmd.Dir = e.prefix
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return true, strings.TrimSpace(string(out))
}

// Status reports whether nginx is running.
func (e *Engine) Status() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return "running"
	}
	return "stopped"
}

// PID returns nginx's process ID, or 0.
func (e *Engine) PID() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return 0
	}
	return e.cmd.Process.Pid
}

// LastError returns why the last start failed.
func (e *Engine) LastError() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastErr
}

// StopStale kills an nginx that a previous run left behind.
//
// After a crash or a forced quit the old nginx still holds its ports, and
// the next start would fail to bind them with no way for the user to tell
// why. Only the pid file under this engine's prefix is consulted, so no
// other nginx on the machine is touched.
func StopStale(prefix string) {
	pidFile := filepath.Join(prefix, "logs", "nginx.pid")
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	// The file is stale by now: a clean exit removes it.
	os.Remove(pidFile)
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	if pidAlive(pid) {
		killTree(pid)
	}
}
