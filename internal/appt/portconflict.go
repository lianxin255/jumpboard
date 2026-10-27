package appt

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lianxin255/jumpboard/internal/engine"
	"github.com/lianxin255/jumpboard/internal/model"
)

// This file turns "the port cannot be bound" into an answer a person can
// act on: who is sitting on the port, and which nearby port is free.
// Nothing here kills a process or changes a rule — the occupier is named
// for reading, and the suggestion is a button that fills in a field.

// procEntry is one process found holding a port.
type procEntry struct {
	PID  string
	Name string
}

// text renders the entry for an error message: the name when the system
// tools could say it, the pid always, so the user can look the rest up.
func (p procEntry) text() string {
	if p.Name == "" {
		return "(pid " + p.PID + ")"
	}
	return p.Name + " (pid " + p.PID + ")"
}

// holdersText renders the holders for one message: one process plainly,
// two joined, more than two as the first two plus a count — a port crowded
// enough for a list is a port the message should stay short about.
func holdersText(ps []procEntry) string {
	switch len(ps) {
	case 0:
		return ""
	case 1:
		return ps[0].text()
	case 2:
		return ps[0].text() + "、" + ps[1].text()
	default:
		return ps[0].text() + "、" + ps[1].text() + fmt.Sprintf(" 等 %d 个进程", len(ps))
	}
}

// runTool runs a system listing tool with a deadline and returns its
// output. lsof exits 1 when it finds nothing, which is an answer, not a
// failure: only a missing tool, a timeout or empty output is an error.
//
// The execution step is an interface so tests can inject a stubbed tool
// result without mocking exec. The production value is the real command.
type toolExecuter func(ctx context.Context, name string, args []string) ([]byte, error)

var defaultToolExecuter toolExecuter = func(ctx context.Context, name string, args []string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, path, args...).Output()
}

func runTool(ctx context.Context, name string, args []string) (string, error) {
	out, err := defaultToolExecuter(ctx, name, args)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return "", errors.New("empty output")
	}
	return string(out), nil
}

// withToolExecuter changes which command runs, for the tests: a fake
// answers with the bytes to parse — the lsof -F stream, or netstat's rows —
// without a process ever starting. The restore puts the real one back.
func withToolExecuter(f toolExecuter) func() {
	prev := defaultToolExecuter
	defaultToolExecuter = f
	return func() { defaultToolExecuter = prev }
}

// bindable reports whether a protocol's listeners can take the port right
// now. The engine binds TCP for the stream and web rules and UDP for the
// UDP ones, so the probe binds the same one.
func bindable(proto model.Protocol, port int) bool {
	if proto == model.UDP {
		return portFreeUDP(port)
	}
	return portFree(port)
}

// protoName is the protocol as lsof and netstat spell it.
func protoName(proto model.Protocol) string {
	if proto == model.UDP {
		return "UDP"
	}
	return "TCP"
}

// clash reports whether two protocols cannot share a listen port: web
// rules of one protocol can, the way the engine groups their routes, and
// everything else — stream against stream, web against stream, HTTP
// against HTTPS — cannot.
func clash(a, b model.Protocol) bool {
	return !(!a.IsStream() && !b.IsStream() && a == b)
}

// freePortWindow is how far from the wanted port the suggestion walks, up
// and down. Near keeps a suggestion readable — 9000 begets 9001, not a
// random five-digit number — and the window bounds the walk when a busy
// machine has a whole range taken.
const freePortWindow = 50

// findFreePort walks outward from start and returns the first port free
// accepts, 0 when nothing nearby does. Up comes before down at every step,
// so the suggestion reads as "the next one" the way a person counts ports.
func findFreePort(start int, free func(int) bool) int {
	for d := 1; d <= freePortWindow; d++ {
		if p := start + d; model.ValidPort(p) && free(p) {
			return p
		}
		if p := start - d; model.ValidPort(p) && free(p) {
			return p
		}
	}
	return 0
}

// suggestPort returns a free port near start for the protocol's listeners,
// 0 when the neighborhood is full. Every mapping's listen port is stepped
// over, whatever its state: a suggestion that collided with a saved rule
// would only move the error one save later.
func (a *App) suggestPort(start int, proto model.Protocol) int {
	taken := map[int]bool{}
	if mappings, err := a.Store.ListMappings(); err == nil {
		for _, m := range mappings {
			if m.ListenPort != 0 {
				taken[m.ListenPort] = true
			}
		}
	}
	return findFreePort(start, func(p int) bool {
		return !taken[p] && bindable(proto, p)
	})
}

// engineHoldsFor reports whether the running engine holds the port for the
// mapping itself. Resaving a live rule on the port it already listens on
// is the everyday edit, not a conflict: the apply swaps the listener under
// the port instead of failing to take it.
func (a *App) engineHoldsFor(mappingID string, port int) bool {
	if mappingID == "" {
		return false
	}
	suffix := ":" + strconv.Itoa(port)
	for _, st := range a.RuleStatus() {
		if st.MappingID == mappingID && st.Listening && strings.HasSuffix(st.Listen, suffix) {
			return true
		}
	}
	return false
}

// claimant names the saved mapping that already owns a listen port, or nil
// when no mapping claims it. excludeID is the mapping being edited, whose
// own port is not a conflict with itself; web rules of one protocol may
// share a port, so they are no claim on each other either.
func (a *App) claimant(port int, proto model.Protocol, excludeID string) *model.Mapping {
	mappings, err := a.Store.ListMappings()
	if err != nil {
		return nil
	}
	for i, m := range mappings {
		if m.ID == excludeID || m.ListenPort != port || !clash(m.Protocol, proto) {
			continue
		}
		return &mappings[i]
	}
	return nil
}

// probeListenPort asks whether the form can listen on the port, and when
// it cannot, says why in one sentence: the mapping that owns the port, or
// the process that holds it — named when the system tools can, unnamed
// when they cannot — plus the nearest free port to move to. excludeID is
// the mapping being edited; "" for a new one.
func (a *App) probeListenPort(port int, proto model.Protocol, excludeID string) (why string, suggest int) {
	if port <= 0 || a.engineHoldsFor(excludeID, port) || bindable(proto, port) {
		return "", 0
	}
	suggest = a.suggestPort(port, proto)
	if c := a.claimant(port, proto, excludeID); c != nil {
		return fmt.Sprintf("端口 %d 已被映射「%s」使用", port, c.Name), suggest
	}
	if who := holdersText(whoHolds(port, protoName(proto))); who != "" {
		return fmt.Sprintf("端口 %d 已被 %s 占用", port, who), suggest
	}
	return fmt.Sprintf("端口 %d 已被其他程序占用", port), suggest
}

// bindHint is the sentence added to a bind failure from apply or preflight:
// who holds the port, and the nearest port that is free. The raw error
// stays the error — this is the line under the button, and the context the
// AI diagnosis reads.
func (a *App) bindHint(err error) string {
	var bind *engine.ErrBind
	if !errors.As(err, &bind) {
		return ""
	}
	proto := model.TCP
	if bind.Proto == "UDP" {
		proto = model.UDP
	}

	// Two enabled mappings on one port reach the bind only as the second
	// one failing on the first one's listener — the engine's preflight
	// reports the pair, and the bar should say the same thing. A lone
	// mapping on the port is the failing rule itself: reapplying it swaps
	// the listener under the port rather than binding against it, so the
	// holder is someone else and the process lookup says who.
	var owners []string
	if mappings, mErr := a.Store.ListMappings(); mErr == nil {
		for _, m := range mappings {
			if m.Enabled && m.ListenPort == bind.Port && clash(m.Protocol, proto) {
				owners = append(owners, m.Name)
			}
		}
	}
	var hint string
	who := holdersText(whoHolds(bind.Port, bind.Proto))
	switch {
	case len(owners) >= 2:
		hint = fmt.Sprintf("端口 %d 被「%s」和「%s」同时监听", bind.Port, owners[0], owners[1])
	case who != "":
		hint = fmt.Sprintf("端口 %d 已被 %s 占用", bind.Port, who)
	default:
		hint = fmt.Sprintf("端口 %d 已被其他程序占用", bind.Port)
	}
	if p := a.suggestPort(bind.Port, proto); p != 0 {
		hint += fmt.Sprintf("，可改用 %d", p)
	}
	return hint
}

// FormPortFix returns the free port the last save offered for a taken
// listen port, 0 when there was none. The number is the form button's
// label and its action in one: 改用 9002.
func (a *App) FormPortFix() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.formPortFix
}

// ClearFormPortFix drops the offered port, once it is used or passed by.
func (a *App) ClearFormPortFix() {
	a.mu.Lock()
	a.formPortFix = 0
	a.mu.Unlock()
}

// UseFormPortFix fills the form's listen port with the port the last save
// offered, and retires the error it answered: a button that fixes the port
// must not leave the field red under the port it just replaced. The form's
// own frame is writing its values every frame, so this runs on the main
// thread, as the button does.
func (a *App) UseFormPortFix() {
	a.mu.Lock()
	if a.formPortFix != 0 {
		a.form.ListenPort = strconv.Itoa(a.formPortFix)
	}
	a.formPortFix = 0
	if a.formErrs != nil {
		delete(a.formErrs, "ListenPort")
	}
	a.mu.Unlock()
	a.notify()
}

// withBindHint returns the failure text with a hint line under it. The
// error itself is the truth, the hint the reading; a plain error has
// nothing to add.
func withBindHint(err error, hint string) string {
	if hint == "" {
		return err.Error()
	}
	return err.Error() + " — " + hint
}
