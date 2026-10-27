package e2e

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// termLibReady reports whether the terminal emulator's library can load.
// It is an environment dependency — a first-run download, or a file
// MYGO_GHOSTTY_VT names — so the terminal tests skip rather than fail
// where it cannot: the feature's own logic is covered by the tests that
// drive it where the library is present, and the CI runners download it.
func termLibReady(t *testing.T) {
	t.Helper()
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty-vt unavailable: %v", err)
	}
}

// TestTerminalShellRuns waits for the shell the page starts, types at it
// through the view, and reads the echo back — the whole terminal path:
// page, lazy session, plugin view.
func TestTerminalShellRuns(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)

	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	// The shell starts off the frame (the library may be a first-run
	// download); frames until the session answers with a prompt.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("终端创建中") {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if !tt.HasText("终端不可用") {
			break
		}
		t.Fatalf("the terminal should have started: %v", tt.Texts())
	}
	if tt.HasText("终端创建中") {
		t.Fatalf("the shell never started: %v", tt.Texts())
	}

	// A click on the terminal gives it the keyboard focus — AutoFocus
	// fights the sidebar tab the page was opened with — and then what is
	// typed goes to the shell.
	if r, ok := tt.Find("Terminal"); ok {
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Type("echo jb-term-ok")
	tt.Key(0, ui.KeyEnter)
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if strings.Contains(v.ShellTextForTest(), "jb-term-ok") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the shell never echoed: %q", v.ShellTextForTest())
}

// TestTerminalProbeConnects drives the probe pane against a fake upstream:
// a rule forwarding to a local listener, a connection through the pane,
// and the greeting the listener writes shows in the terminal.
func TestTerminalProbeConnects(t *testing.T) {
	termLibReady(t)
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		conn, err := upstream.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("jb-probe-ready\r\n"))
		buf := make([]byte, 256)
		for {
			n, err := conn.Read(buf)
			if n > 0 && strings.Contains(string(buf[:n]), "probe-hello") {
				_, _ = conn.Write([]byte("jb-probe-echo\r\n"))
			}
			if err != nil {
				return
			}
		}
	}()

	a := testApp(t)
	m := streamRule(t, "探测目标")
	m.UpstreamHost, m.UpstreamPort = "127.0.0.1", upstream.Addr().(*net.TCPAddr).Port
	if _, err := a.Store.CreateMapping(m); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)
	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("端口探测"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the picker to offer the rule", func() bool {
		return tt.HasText("探测目标 — 127.0.0.1:" + strconv.Itoa(m.ListenPort))
	})
	if err := tt.Click("连接"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the greeting to show", func() bool {
		return strings.Contains(v.ProbeTextForTest(), "jb-probe-ready")
	})

	// What is typed goes to the port raw: the fake upstream echoes on the
	// agreed word.
	tt.Type("probe-hello")
	tt.Key(0, ui.KeyEnter)
	waitFor(t, tt, "the upstream's answer", func() bool {
		return strings.Contains(v.ProbeTextForTest(), "jb-probe-echo")
	})

	// The connection is news the log carries.
	found := false
	_, logs, _, _, _, _, _, _ := a.Snapshot()
	for _, l := range logs {
		if strings.Contains(l.Message, "端口探测已连接") {
			found = true
		}
	}
	if !found {
		t.Error("the probe connection should be logged")
	}
}
