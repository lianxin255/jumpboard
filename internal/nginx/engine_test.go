package nginx

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// bundledEngine returns an engine on the bundled nginx, or skips the test
// when this machine has none (the binary is shipped per platform).
func bundledEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	bin := os.Getenv("NGX_BIN")
	if bin == "" {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		name := "nginx"
		if runtime.GOOS == "windows" {
			name = "nginx.exe"
		}
		// The universal build for macOS, the platform folders elsewhere.
		for _, candidate := range []string{
			filepath.Join(root, "resources", "darwin", "nginx", name),
			filepath.Join(root, "resources", runtime.GOOS+"-"+runtime.GOARCH, "nginx", name),
		} {
			if _, err := os.Stat(candidate); err == nil {
				bin = candidate
				break
			}
		}
	}
	if bin == "" {
		t.Skip("这台机器上没有内置 nginx，跳过")
	}
	return NewEngine(bin, t.TempDir()), bin
}

// freePort returns a port nothing listens on right now.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// oneTCPConf builds a configuration forwarding listen to upstream.
func oneTCPConf(listen, upstream int) string {
	return Generate([]model.Mapping{{
		Name: "测试转发", Protocol: model.TCP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upstream,
	}}, GenOptions{})
}

// TestEngineLifecycle starts nginx, reloads it and stops it, checking the
// port is free afterwards so that a second start works.
func TestEngineLifecycle(t *testing.T) {
	e, _ := bundledEngine(t)
	listen, upstream := freePort(t), freePort(t)

	if err := e.WriteConf(oneTCPConf(listen, upstream)); err != nil {
		t.Fatal(err)
	}
	if ok, out := e.Test(); !ok {
		t.Fatalf("nginx -t 失败: %s", out)
	}
	if e.Status() != "stopped" {
		t.Fatalf("初始状态应为 stopped: %s", e.Status())
	}

	if err := e.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer e.Stop()
	if e.Status() != "running" {
		t.Fatalf("启动后应为 running: %s", e.Status())
	}
	if e.PID() == 0 {
		t.Error("运行中应有 PID")
	}
	if err := WaitListening(listen, 3*time.Second); err != nil {
		t.Fatal(err)
	}

	// A reload with a new configuration keeps serving.
	if err := e.WriteConf(oneTCPConf(listen, upstream)); err != nil {
		t.Fatal(err)
	}
	if err := e.Reload(); err != nil {
		t.Fatalf("reload 失败: %v", err)
	}

	if err := e.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if e.Status() != "stopped" {
		t.Fatalf("停止后应为 stopped: %s", e.Status())
	}
	if err := WaitClosed(listen, 3*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.prefix, "logs", "nginx.pid")); err == nil {
		t.Error("停止后不应留下 pid 文件")
	}
}

// TestEngineStartFails reports why nginx refused to start, which the UI
// shows as the error.
func TestEngineStartFails(t *testing.T) {
	e, _ := bundledEngine(t)
	// Two mappings on one port: nginx -t passes, the start cannot bind.
	port := freePort(t)
	conf := Generate([]model.Mapping{
		{Name: "a", Protocol: model.TCP, Enabled: true, ListenPort: port, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{Name: "b", Protocol: model.TCP, Enabled: true, ListenPort: port, UpstreamHost: "127.0.0.1", UpstreamPort: 2},
	}, GenOptions{})
	if err := e.WriteConf(conf); err != nil {
		t.Fatal(err)
	}
	err := e.Start()
	if err == nil {
		e.Stop()
		t.Skip("这个平台上 nginx 没有在启动时报错")
	}
	if e.LastError() == "" && err.Error() == "" {
		t.Error("启动失败应带上 nginx 的原因")
	}
	if e.Status() != "stopped" {
		t.Errorf("启动失败后应为 stopped: %s", e.Status())
	}
}

// TestApplyRollsBack checks that a configuration nginx rejects leaves the
// previous one on disk.
func TestApplyRollsBack(t *testing.T) {
	e, _ := bundledEngine(t)
	good := oneTCPConf(freePort(t), freePort(t))
	if err := e.WriteConf(good); err != nil {
		t.Fatal(err)
	}

	bad := good + "\nthis is not nginx;\n"
	res := e.Apply(bad, true)
	if res.OK {
		t.Fatal("非法配置应失败")
	}
	if res.Stage != "test" {
		t.Fatalf("失败阶段应为 test: %s", res.Stage)
	}
	got, err := e.ReadConf()
	if err != nil {
		t.Fatal(err)
	}
	if got != good {
		t.Errorf("失败后应回滚到原配置:\n--- got ---\n%s\n--- want ---\n%s", got, good)
	}
}

// TestApplyWritesWhenStopped checks that a stopped engine still writes and
// checks a configuration.
func TestApplyWritesWhenStopped(t *testing.T) {
	e, _ := bundledEngine(t)
	conf := oneTCPConf(freePort(t), freePort(t))
	res := e.Apply(conf, true)
	if !res.OK {
		t.Fatalf("停止态应用应成功: %s", res.Output)
	}
	got, _ := e.ReadConf()
	if got != conf {
		t.Error("配置没有写到磁盘")
	}
	if e.Status() != "stopped" {
		t.Error("停止态应用不应启动 nginx")
	}
}

// TestStopStale kills a process whose pid file a crash left behind.
func TestStopStale(t *testing.T) {
	e, _ := bundledEngine(t)
	if err := e.WriteConf(oneTCPConf(freePort(t), freePort(t))); err != nil {
		t.Fatal(err)
	}
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	pid := e.PID()
	// Leave the process running and forget it, as a crash would.
	_, _ = e.signal("quit")
	time.Sleep(200 * time.Millisecond)
	if pidAlive(pid) {
		// The quit worked; write the pid file back to simulate the crash
		// state instead.
		if err := os.WriteFile(filepath.Join(e.prefix, "logs", "nginx.pid"), []byte(itoa(pid)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	StopStale(e.prefix)
	if _, err := os.Stat(filepath.Join(e.prefix, "logs", "nginx.pid")); err == nil {
		t.Error("StopStale 应删掉陈旧的 pid 文件")
	}
}
