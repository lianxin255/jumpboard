package ui

import (
	"fmt"
	"net"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/model"
)

// terminalPage is the in-app terminal: a local shell beside the rules, or
// a probe session wired straight to a forwarded port, whose bytes go
// through the real listener the engine owns. SSH is out of scope here on
// purpose; serial ports and tabs are a later round.
func (v *View) terminalPage(c *ui.Context, t Tokens) {
	a := v.App
	mappings, _, _, _, _, _, _, _ := a.Snapshot()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "终端").FontSize(TitleSize).Bold().TextColor(t.FG)
			ui.Text(c, "在应用里跑本地 shell，或对转发端口做 TCP/HTTP 探测").FontSize(SubtitleSz).TextColor(t.Muted)
		})
		ui.Segmented(c, &v.termMode, "本地终端", "端口探测").Label("会话类型")

		if v.termMode == 0 {
			v.shellPane(c, t)
			return
		}
		v.probePane(c, t, mappings)
	})
}

// shellPane draws the local shell session. The shell is started off the
// frame — the emulator library may be a first-run download, and no frame
// should wait on one — so what the frame reads is a snapshot the starter
// goroutine wrote behind termMu.
func (v *View) shellPane(c *ui.Context, t Tokens) {
	v.termMu.Lock()
	shell, err, starting, done := v.shell, v.shellErr, v.shellStarting, v.shellDone
	v.termMu.Unlock()

	if err != "" {
		v.termMessageCard(c, t, "终端不可用", err,
			"终端需要随包的 libghostty-vt（由 mygo 构建自动放进应用）；开发运行请用 mygo dev，或设置 MYGO_GHOSTTY_VT 指向库文件。")
		return
	}
	if shell == nil {
		if !starting {
			v.startShell()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("终端创建中")
			ui.Text(c, "首次运行需要下载终端模拟库（1–2 MB），之后直接启动。").
				FontSize(11).TextColor(t.Muted)
		})
		return
	}
	if done {
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().
			Padding(8, 10).Radius(RadiusPanel).
			Background(t.Warning.Alpha(0.08)).Border(1, t.Warning.Alpha(0.45)).
			Children(func() {
				ui.Text(c, "shell 已退出").FontSize(12).TextColor(t.FG2).Grow(1)
				if ui.Button(c, "重开终端").Clicked() {
					v.restartShell()
				}
			})
	}
	terminal.View(c, shell).Fill().AutoFocus().Grow(1)
}

// startShell starts the local shell once; a second call while one starts
// or runs does nothing. The goroutine writes the session fields behind
// termMu and asks for a frame, never the view's other state.
func (v *View) startShell() {
	a := v.App
	v.termMu.Lock()
	if v.shellStarting || v.shell != nil {
		v.termMu.Unlock()
		return
	}
	v.shellStarting, v.shellErr, v.shellDone = true, "", false
	v.termMu.Unlock()
	go func() {
		term, err := terminal.New(terminal.Options{
			OnExit: func(int) {
				v.termMu.Lock()
				v.shellDone = true
				v.termMu.Unlock()
				a.Change()
			},
		})
		v.termMu.Lock()
		v.shellStarting = false
		if err != nil {
			v.shellErr = err.Error()
		} else {
			v.shell = term
			a.OnClose(func() { _ = term.Close() })
		}
		v.termMu.Unlock()
		a.Change()
	}()
}

// restartShell hangs the exited shell up — it frees the library's side of
// the session — and starts a fresh one, keeping the old screen out of the
// new session's scrollback.
func (v *View) restartShell() {
	v.termMu.Lock()
	old := v.shell
	v.shell, v.shellDone = nil, false
	v.termMu.Unlock()
	if old != nil {
		old.Close()
	}
	v.startShell()
}

// probePane draws the probe session: pick a forwarded port, connect, and
// whatever the port says shows in the terminal while what is typed goes
// to it raw. Connection outcomes land in the operation log too.
func (v *View) probePane(c *ui.Context, t Tokens, mappings []model.Mapping) {
	type target struct {
		m     model.Mapping
		label string
	}
	var targets []target
	options := make([]string, 0, len(mappings))
	for _, m := range mappings {
		if m.Enabled && m.ListenPort > 0 {
			tg := target{m, fmt.Sprintf("%s — 127.0.0.1:%d（%s）", m.Name, m.ListenPort, m.Protocol.Label())}
			targets = append(targets, tg)
			options = append(options, tg.label)
		}
	}
	if len(targets) == 0 {
		v.termMessageCard(c, t, "没有可探测的端口",
			"探测需要一条已启用、带监听端口的映射。",
			"去「映射」建一条并应用配置，再回到这里。")
		return
	}
	pick := targets[len(targets)-1].label
	for _, tg := range targets {
		if tg.label == v.probePick {
			pick = tg.label
			break
		}
	}
	var chosen target
	for _, tg := range targets {
		if tg.label == pick {
			chosen = tg
			break
		}
	}

	v.termMu.Lock()
	probe, addr, dialing, perr, done := v.probe, v.probeAddr, v.probeDialing, v.probeErr, v.probeDone
	v.termMu.Unlock()

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &pick, options).Grow(1).Label("目标端口").
			Tooltip("已启用且带监听端口的映射；探测走真实的转发链路。")
		if probe != nil && !done {
			if ui.Button(c, "断开").Clicked() {
				probe.Close()
			}
			if chosen.m.Protocol == model.HTTP {
				if ui.Button(c, "发送 HTTP 探测").Tooltip("向端口发送 GET / 并显示应答").Clicked() {
					probe.Send([]byte(fmt.Sprintf(
						"GET / HTTP/1.1\r\nHost: 127.0.0.1:%d\r\nUser-Agent: JumpBoard-probe\r\nConnection: close\r\n\r\n",
						chosen.m.ListenPort)))
				}
			}
			ui.Text(c, "已连接 "+addr).FontSize(11).TextColor(t.Muted)
		} else if dialing {
			ui.Spinner(c).Label("连接中")
		} else {
			if ui.PrimaryButton(c, "连接").Clicked() {
				v.probePick = pick
				v.connectProbe(chosen.m)
			}
		}
	})
	if perr != "" && probe == nil {
		ui.Text(c, perr).FontSize(11).TextColor(t.Danger).Wrap()
	}
	if probe == nil {
		// Still dialing (the row spins), or not connected yet: no
		// terminal exists to show, only the hint.
		if !dialing {
			ui.Text(c, "连接后输入的内容会原样发往该端口，端口的应答显示在这里。").
				FontSize(11).TextColor(t.Muted).Wrap()
		}
		return
	}
	if done {
		ui.Text(c, "连接已断开").FontSize(11).TextColor(t.Warning)
	}
	terminal.View(c, probe).Fill().AutoFocus().Grow(1)
}

// connectProbe dials a forwarded port and wires the connection to a new
// terminal: what the port writes shows, what is typed goes to it. The
// dial and the terminal's first screen wait on no frame; the goroutine
// writes the session fields behind termMu and reports through the log
// and a frame.
func (v *View) connectProbe(m model.Mapping) {
	a := v.App
	addr := fmt.Sprintf("127.0.0.1:%d", m.ListenPort)
	v.termMu.Lock()
	if v.probeDialing {
		v.termMu.Unlock()
		return
	}
	v.probeDialing, v.probeErr, v.probeDone = true, "", false
	old := v.probe
	v.probe = nil
	v.termMu.Unlock()
	if old != nil {
		old.Close()
	}
	go func() {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			v.termMu.Lock()
			v.probeDialing = false
			v.probeErr = "连接失败：" + err.Error()
			v.termMu.Unlock()
			a.Log.Error("端口探测 %s（%s）失败: %v", addr, m.Name, err)
			_ = a.Reload()
			return
		}
		term, err := terminal.New(terminal.Options{
			Conn: conn,
			OnExit: func(int) {
				v.termMu.Lock()
				v.probeDone = true
				v.termMu.Unlock()
				a.Log.Info("端口探测 %s（%s）已断开", addr, m.Name)
				_ = a.Reload()
			},
		})
		if err != nil {
			conn.Close()
			v.termMu.Lock()
			v.probeDialing = false
			v.probeErr = "终端不可用：" + err.Error()
			v.termMu.Unlock()
			a.Log.Error("端口探测 %s（%s）无法开终端: %v", addr, m.Name, err)
			_ = a.Reload()
			return
		}
		v.termMu.Lock()
		v.probeDialing = false
		v.probe, v.probeAddr = term, addr
		v.termMu.Unlock()
		a.OnClose(func() { _ = term.Close() })
		a.Log.Info("端口探测已连接 %s（%s）", addr, m.Name)
		_ = a.Reload()
	}()
}

// termMessageCard is the pane's answer when there is nothing to drive:
// why, and what would change it.
func (v *View) termMessageCard(c *ui.Context, t Tokens, title, why, hint string) {
	ui.Box(c).FillWidth().Grow(1).Padding(16).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, title).FontSize(13).Bold().TextColor(t.FG)
			ui.Text(c, why).FontSize(12).TextColor(t.FG2).Wrap()
			ui.Text(c, hint).FontSize(11).TextColor(t.Muted).Wrap()
		})
	})
}
