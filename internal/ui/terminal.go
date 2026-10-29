package ui

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/serial"
)

// termKind is what a session tab carries.
type termKind int

// Session kinds.
const (
	termShell termKind = iota
	termProbe
	termSerial
)

// termLabel is the kind's name, for the picker and the tab's title.
func (k termKind) label() string {
	switch k {
	case termShell:
		return "本地终端"
	case termProbe:
		return "端口探测"
	default:
		return "串口"
	}
}

// serialRateDefault is the picker's starting rate, 115200.
const serialRateDefault = "115200"

// serialRateNames are the picker's options, in AllRates' order.
var serialRateNames = func() []string {
	rates := serial.AllRates()
	out := make([]string, len(rates))
	for i, r := range rates {
		out[i] = strconv.Itoa(int(r))
	}
	return out
}()

// rateFromPick is the rate a pick names; the last one offered if it names
// none, so a pick left empty or unknown still opens at a rate.
func rateFromPick(pick string) serial.Baud {
	rates := serial.AllRates()
	for i, r := range rates {
		if serialRateNames[i] == pick {
			return r
		}
	}
	return rates[len(rates)-1]
}

// tabSession is one terminal tab: what it is, what it holds, and its
// own mutable state. Every field lives behind v.termMu — the frame reads
// a copy, the goroutine that started the session writes it — and the
// pane each kind draws its own setup around.
type tabSession struct {
	id     int
	kind   termKind
	name   string // the tab's title, stable once the tab exists
	term   *terminal.Terminal
	termLn string // what the session is connected to, "" before it lands
	// starting is true from the moment the tab asked for a session until
	// term or err is set; done marks an exited session.
	starting bool
	err      string
	done     bool

	// probePick, serialPick and serialRatePick are the tab's own choices, so
	// two probe tabs can watch two ports without tripping over each
	// other. serialRatePick is one of AllRateNames(), what the picker's
	// Select holds.
	probePick      string
	serialPick     string
	serialRatePick string
	serialLoaded   bool
	serialLoad     bool
	serialPorts    []serial.Port
}

// terminalPage is the in-app terminal: tabs of sessions — a local shell,
// a probe of a forwarded port, or a serial device — which the user opens
// and closes independently, and which quit hangs up all of. SSH is out of
// scope here on purpose.
func (v *View) terminalPage(c *ui.Context, t Tokens) {
	a := v.App
	mappings, _, _, _, _, _, _, _ := a.Snapshot()
	v.initTermOnce()

	v.termMu.Lock()
	tabs := append([]*tabSession(nil), v.termTabs...)
	active := v.termActive
	v.termMu.Unlock()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		// The window bar says the page's name and what it is for; the
		// page starts with its own toolbar.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &v.termNewKind, termShell.label(), termProbe.label(), termSerial.label()).
				Label("新会话类型")
			if ui.PrimaryButton(c, "新建会话").Clicked() {
				v.newTab(termKind(v.termNewKind), mappings)
			}
			if len(tabs) > 0 && ui.Button(c, "全部关闭").Clicked() {
				v.closeAllTabs()
			}
		})

		if len(tabs) == 0 {
			v.termMessageCard(c, t, "还没有会话",
				"选一个会话类型再点「新建会话」。",
				"本地终端跑你的 shell；端口探测连到转发的端口；串口接本机的串行设备。")
			return
		}

		// The tab row: one button per tab, its title, and a × that closes
		// just that one. The active tab is the one the pane below draws.
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().
			Padding(4, 6).Radius(RadiusPanel).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			for i, s := range tabs {
				i, s := i, s
				on := i == active
				tab := ui.Button(c, s.name).FontSize(NoteSize)
				if on {
					tab.Bold()
				}
				if tab.Clicked() {
					v.activateTab(i)
				}
				closeBtn := ui.Button(c, "×").FontSize(NoteSize).Width(22).
					Label("关闭-" + s.name).Tooltip("关闭这个会话")
				if closeBtn.Clicked() {
					v.closeTab(s.id)
				}
			}
		})
		if active >= len(tabs) {
			v.termMessageCard(c, t, "没有可显示的会话",
				"选一个标签，或新建一个。", "标签已全部关闭。")
			return
		}
		s := tabs[active]
		switch s.kind {
		case termShell:
			v.shellPane(c, t, s)
		case termProbe:
			v.probePane(c, t, s, mappings)
		case termSerial:
			v.serialPane(c, t, s)
		}
	})
}

// initTermOnce registers the quit hook that hangs up every session. The
// app knows nothing of terminals; the view registers the cleanup through
// appt's own hook, once, on the first frame of the page.
func (v *View) initTermOnce() {
	v.termInit.Do(func() {
		v.App.OnClose(func() {
			v.termMu.Lock()
			tabs := v.termTabs
			v.termMu.Unlock()
			for _, s := range tabs {
				v.hangUp(s)
			}
		})
	})
}

// newTab opens a session tab of the kind the picker holds, keeping the
// active tab where it was until the new one is ready to be seen.
func (v *View) newTab(kind termKind, mappings []model.Mapping) {
	v.termMu.Lock()
	v.termSeq++
	id := v.termSeq
	name := kind.label() + "-" + strconv.Itoa(id)
	s := &tabSession{id: id, kind: kind, name: name, serialRatePick: serialRateDefault}
	switch kind {
	case termProbe:
		// The first enabled mapping is a sensible default for a probe.
		for _, m := range mappings {
			if m.Enabled && m.ListenPort > 0 {
				s.probePick = probeLabel(m)
				break
			}
		}
	}
	v.termTabs = append(v.termTabs, s)
	v.termActive = len(v.termTabs) - 1
	v.termMu.Unlock()
	v.App.Change()
}

// activateTab shows the tab at i.
func (v *View) activateTab(i int) {
	v.termMu.Lock()
	if i >= 0 && i < len(v.termTabs) {
		v.termActive = i
	}
	v.termMu.Unlock()
	v.App.Change()
}

// closeTab closes the session with id and drops the tab, keeping whatever
// tab is left active.
func (v *View) closeTab(id int) {
	v.termMu.Lock()
	var gone *tabSession
	out := v.termTabs[:0]
	for _, s := range v.termTabs {
		if s.id == id {
			gone = s
			continue
		}
		out = append(out, s)
	}
	v.termTabs = out
	if v.termActive >= len(out) {
		v.termActive = len(out) - 1
	}
	v.termMu.Unlock()
	if gone != nil {
		v.hangUp(gone)
	}
	v.App.Change()
}

// closeAllTabs hangs up and drops every tab.
func (v *View) closeAllTabs() {
	v.termMu.Lock()
	tabs := v.termTabs
	v.termTabs = nil
	v.termActive = 0
	v.termMu.Unlock()
	for _, s := range tabs {
		v.hangUp(s)
	}
	v.App.Change()
}

// hangUp closes the tab's session, if any. terminal.Close is idempotent,
// so a session already exited is freed again without complaint.
func (v *View) hangUp(s *tabSession) {
	v.termMu.Lock()
	term := s.term
	s.term, s.starting = nil, false
	v.termMu.Unlock()
	if term != nil {
		_ = term.Close()
	}
}

// shellPane draws a shell tab. The shell starts off the frame — the
// emulator library may be a first-run download — so the frame reads a
// snapshot the starter goroutine wrote behind termMu.
func (v *View) shellPane(c *ui.Context, t Tokens, s *tabSession) {
	v.termMu.Lock()
	term, err, starting, done := s.term, s.err, s.starting, s.done
	v.termMu.Unlock()

	if err != "" {
		v.termMessageCard(c, t, "终端不可用", err,
			"终端需要随包的 libghostty-vt（由 mygo 构建自动放进应用）；开发运行请用 mygo dev，或设置 MYGO_GHOSTTY_VT 指向库文件。")
		return
	}
	if term == nil {
		if !starting {
			v.startShell(s)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("终端创建中")
			ui.Text(c, "首次运行需要下载终端模拟库（1–2 MB），之后直接启动。").
				FontSize(CaptionSize).TextColor(t.Muted)
		})
		return
	}
	if done {
		fill, line := statusOutline(t.Warning)
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().
			Padding(8, 10).Radius(RadiusPanel).
			Background(fill).Border(1, line).
			Children(func() {
				ui.Text(c, "shell 已退出").FontSize(NoteSize).TextColor(t.FG2).Grow(1)
				if ui.Button(c, "重开终端").Clicked() {
					v.startShell(s)
				}
			})
	}
	terminal.View(c, term).Fill().AutoFocus().Grow(1)
}

// startShell starts the tab's shell once; a call while one starts or runs
// does nothing. It hangs the old session up first, so a restarted tab
// starts with a clean scrollback.
func (v *View) startShell(s *tabSession) {
	a := v.App
	v.termMu.Lock()
	if s.starting || s.term != nil {
		v.termMu.Unlock()
		return
	}
	s.starting, s.err, s.done = true, "", false
	v.termMu.Unlock()
	v.hangUp(s)
	go func() {
		term, err := terminal.New(terminal.Options{
			OnExit: func(int) {
				v.termMu.Lock()
				s.done = true
				v.termMu.Unlock()
				a.Change()
			},
		})
		v.termMu.Lock()
		s.starting = false
		if err != nil {
			s.err = err.Error()
		} else {
			s.term = term
		}
		v.termMu.Unlock()
		a.Change()
	}()
}

// probeLabel is a mapping's place in the probe picker.
func probeLabel(m model.Mapping) string {
	return fmt.Sprintf("%s — 127.0.0.1:%d（%s）", m.Name, m.ListenPort, m.Protocol.Label())
}

// probePane draws a probe tab: pick a forwarded port, connect, and
// whatever the port says shows in the terminal while what is typed goes
// to it raw. Only the tab the picker sits in is driven, so two tabs can
// watch two ports. Outcomes land in the operation log too.
func (v *View) probePane(c *ui.Context, t Tokens, s *tabSession, mappings []model.Mapping) {
	// A probe is a port probe; the mapping only gives it a name. Only
	// enabled rules that listen somewhere belong in the picker.
	var targets []model.Mapping
	var options []string
	for _, m := range mappings {
		if !m.Enabled || m.ListenPort <= 0 {
			continue
		}
		targets = append(targets, m)
		options = append(options, probeLabel(m))
	}
	if len(targets) == 0 {
		v.termMessageCard(c, t, "没有可探测的端口",
			"没有启用且带监听端口的转发规则。", "先在「映射」页新建一条规则并应用配置。")
		return
	}
	// The pick's own default is an enabled mapping; it may have left the
	// list since, so it falls back to the first one.
	pick := s.probePick
	found := false
	for _, o := range options {
		if o == pick {
			found = true
			break
		}
	}
	if !found {
		pick = options[0]
	}
	var chosen model.Mapping
	for i, tg := range targets {
		if options[i] == pick {
			chosen = tg
			break
		}
	}

	v.termMu.Lock()
	term, termLn, starting, err, done := s.term, s.termLn, s.starting, s.err, s.done
	v.termMu.Unlock()
	if err != "" {
		v.termMessageCard(c, t, "连接失败", err,
			"端口不可达，或终端库不可用；确认转发在跑，再试一次。")
		return
	}

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &pick, options).Grow(1).Label("目标端口").
			Tooltip("已启用且带监听端口的映射；探测走真实的转发链路。")
		if pick != s.probePick {
			s.probePick = pick
		}
		if term != nil && !done {
			if ui.Button(c, "断开").Clicked() {
				v.hangUp(s)
			}
			if chosen.Protocol == model.HTTP {
				if ui.Button(c, "发送 HTTP 探测").Tooltip("向端口发送 GET / 并显示应答").Clicked() {
					term.Send([]byte(fmt.Sprintf(
						"GET / HTTP/1.1\r\nHost: 127.0.0.1:%d\r\nUser-Agent: JumpBoard-probe\r\nConnection: close\r\n\r\n",
						chosen.ListenPort)))
				}
			}
			ui.Text(c, "已连接 "+termLn).FontSize(CaptionSize).TextColor(t.Muted)
		} else if starting {
			ui.Spinner(c).Label("连接中")
		} else if ui.PrimaryButton(c, "连接").Clicked() {
			v.connectProbe(s, chosen)
		}
	})
	if term == nil {
		if !starting {
			ui.Text(c, "连接后输入的内容会原样发往该端口，端口的应答显示在这里。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		}
		return
	}
	if done {
		fill, line := statusOutline(t.Warning)
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().
			Padding(8, 10).Radius(RadiusPanel).Background(fill).Border(1, line).Children(func() {
			ui.Text(c, "连接已断开").FontSize(NoteSize).TextColor(t.FG2).Grow(1)
			if ui.PrimaryButton(c, "重新连接").Clicked() {
				v.connectProbe(s, chosen)
			}
		})
	}
	terminal.View(c, term).Fill().AutoFocus().Grow(1)
}

// connectProbe dials a forwarded port and wires the connection to the
// tab's terminal: what the port writes shows, what is typed goes to it.
// The dial and the terminal's first screen wait on no frame; the
// goroutine writes the tab's fields behind termMu and reports through the
// log and a frame.
func (v *View) connectProbe(s *tabSession, m model.Mapping) {
	a := v.App
	addr := fmt.Sprintf("127.0.0.1:%d", m.ListenPort)
	v.termMu.Lock()
	if s.starting || s.term != nil {
		v.termMu.Unlock()
		return
	}
	s.starting, s.err, s.done = true, "", false
	v.termMu.Unlock()
	go func() {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			v.termMu.Lock()
			s.starting, s.err = false, "连接失败："+err.Error()
			v.termMu.Unlock()
			a.Log.Error("端口探测 %s（%s）失败: %v", addr, m.Name, err)
			_ = a.Reload()
			return
		}
		term, err := terminal.New(terminal.Options{
			Conn: conn,
			OnExit: func(int) {
				v.termMu.Lock()
				s.done = true
				v.termMu.Unlock()
				a.Log.Info("端口探测 %s（%s）已断开", addr, m.Name)
				_ = a.Reload()
			},
		})
		if err != nil {
			conn.Close()
			v.termMu.Lock()
			s.starting, s.err = false, "终端不可用："+err.Error()
			v.termMu.Unlock()
			a.Log.Error("端口探测 %s（%s）无法开终端: %v", addr, m.Name, err)
			_ = a.Reload()
			return
		}
		v.termMu.Lock()
		s.starting, s.term, s.termLn, s.done = false, term, addr, false
		v.termMu.Unlock()
		a.Log.Info("端口探测已连接 %s（%s）", addr, m.Name)
		_ = a.Reload()
	}()
}

// serialPane draws a serial tab: pick a device and a rate, open it, and
// the device's output shows in the terminal while what is typed goes to
// it. A device list the system refuses is a readable note, not an error
// — some machines simply have none.
func (v *View) serialPane(c *ui.Context, t Tokens, s *tabSession) {
	v.termMu.Lock()
	term, starting, err, done := s.term, s.starting, s.err, s.done
	ports, loaded := s.serialPorts, s.serialLoaded
	load := s.serialLoad
	v.termMu.Unlock()

	// The list is loaded once, off the frame; opening a port can block,
	// which is why the list is not the thing that blocks.
	if !loaded {
		if !load {
			v.loadSerialPorts(s)
		}
		ui.Spinner(c).Label("列出串口")
		return
	}
	if len(ports) == 0 {
		v.termMessageCard(c, t, "没有可用的串口",
			"这台机器没有枚举到串行设备。", "插上串口线（USB 转串口、蓝牙串口）后刷新；macOS 的蓝牙口可能是 /dev/cu.Bluetooth-Incoming-Port。")
		return
	}
	// The device and rate are the frame's, read and written on its own
	// thread: the goroutine that lists the ports writes the list once,
	// never the pick. A pick left empty when the list lands falls back
	// to the first device.
	names := make([]string, 0, len(ports))
	for _, p := range ports {
		names = append(names, p.Name)
	}
	pick := s.serialPick
	found := false
	for _, n := range names {
		if n == pick {
			found = true
			break
		}
	}
	if !found {
		pick = names[0]
	}
	ratePick := s.serialRatePick
	if ratePick == "" {
		ratePick = serialRateDefault
	}

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &pick, names).Grow(1).Label("设备").
			Tooltip("本机串行设备；macOS 用 /dev/cu.*，Windows 用 COM 名。")
		ui.Select(c, &ratePick, serialRateNames).Width(110).Label("波特率")
		if term != nil && !done {
			if ui.Button(c, "断开").Clicked() {
				v.hangUp(s)
			}
			ui.Text(c, "已连接 "+pick).FontSize(CaptionSize).TextColor(t.Muted)
		} else if starting {
			ui.Spinner(c).Label("打开中")
		} else if ui.PrimaryButton(c, "打开串口").Clicked() {
			v.openSerial(s, pick, rateFromPick(ratePick))
		}
	})
	// The picks the user made in this frame belong to the tab from now on.
	if pick != s.serialPick {
		s.serialPick = pick
	}
	if ratePick != s.serialRatePick {
		s.serialRatePick = ratePick
	}
	if err != "" && term == nil {
		ui.Text(c, err).FontSize(CaptionSize).TextColor(t.Danger).Wrap()
	}
	if term == nil {
		if !starting {
			ui.Text(c, "串口以原始 8-N-1 打开：设备输出显示在这里，输入原样发往设备。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		}
		return
	}
	if done {
		fill, line := statusOutline(t.Warning)
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().
			Padding(8, 10).Radius(RadiusPanel).Background(fill).Border(1, line).Children(func() {
			ui.Text(c, "串口已断开").FontSize(NoteSize).TextColor(t.FG2).Grow(1)
			if ui.PrimaryButton(c, "重新打开").Clicked() {
				v.openSerial(s, pick, rateFromPick(ratePick))
			}
		})
	}
	terminal.View(c, term).Fill().AutoFocus().Grow(1)
}

// loadSerialPorts lists the machine's serial devices once. The list
// belongs to the tab, so each serial tab refreshes its own, and a list
// the system refuses leaves a note for the pane rather than a spin.
func (v *View) loadSerialPorts(s *tabSession) {
	a := v.App
	v.termMu.Lock()
	s.serialLoad = true
	v.termMu.Unlock()
	go func() {
		ports := serial.List()
		v.termMu.Lock()
		s.serialLoaded, s.serialLoad = true, false
		s.serialPorts = ports
		if len(ports) > 0 && s.serialPick == "" {
			s.serialPick = ports[0].Name
		}
		v.termMu.Unlock()
		a.Change()
	}()
}

// openSerial opens the device at rate and wires it to the tab's terminal:
// the device's output shows, what is typed goes out. Opening a serial
// port can block (a missing device, a busy driver), so it is off the
// frame; a refusal leaves a readable note in the tab.
func (v *View) openSerial(s *tabSession, path string, rate serial.Baud) {
	a := v.App
	v.termMu.Lock()
	if s.starting || s.term != nil {
		v.termMu.Unlock()
		return
	}
	s.starting, s.err, s.done = true, "", false
	v.termMu.Unlock()
	go func() {
		conn, err := serial.Open(path, rate)
		if err != nil {
			v.termMu.Lock()
			s.starting, s.err = false, "打开串口失败："+err.Error()
			v.termMu.Unlock()
			a.Log.Error("串口 %s 打开失败: %v", path, err)
			_ = a.Reload()
			return
		}
		term, err := terminal.New(terminal.Options{
			Conn: conn,
			OnExit: func(int) {
				v.termMu.Lock()
				s.done = true
				v.termMu.Unlock()
				a.Log.Info("串口 %s 已断开", path)
				_ = a.Reload()
			},
		})
		if err != nil {
			conn.Close()
			v.termMu.Lock()
			s.starting, s.err = false, "终端不可用："+err.Error()
			v.termMu.Unlock()
			a.Log.Error("串口 %s 无法开终端: %v", path, err)
			_ = a.Reload()
			return
		}
		v.termMu.Lock()
		s.starting, s.term, s.termLn, s.done = false, term, path, false
		v.termMu.Unlock()
		a.Log.Info("串口 %s 已打开 @ %d", path, rate)
		_ = a.Reload()
	}()
}

// termMessageCard is the pane's answer when there is nothing to drive:
// why, and what would change it.
func (v *View) termMessageCard(c *ui.Context, t Tokens, title, why, hint string) {
	ui.Box(c).FillWidth().Grow(1).Padding(16).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, title).FontSize(BodySize).Bold().TextColor(t.FG)
			ui.Text(c, why).FontSize(NoteSize).TextColor(t.FG2).Wrap()
			ui.Text(c, hint).FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		})
	})
}
