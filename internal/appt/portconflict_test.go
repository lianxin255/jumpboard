package appt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/engine"
	"github.com/lianxin255/jumpboard/internal/model"
)

// TestFindFreePort walks out from start in both directions and accepts
// the first port free accepts. Up comes before down at every step, so a
// free port past 9001 reads as "the next one" the way a person counts, and
// the window bounds the walk on a machine with a whole range taken.
func TestFindFreePort(t *testing.T) {
	even := func(p int) bool { return p%2 == 0 }

	if got := findFreePort(10, even); got != 12 {
		t.Errorf("findFreePort(10, even) = %d, want 12", got)
	}
	// Up wins the tie at every distance: from 11 the first even port up
	// (12) is nearer to read than the one down (10).
	if got := findFreePort(11, even); got != 12 {
		t.Errorf("findFreePort(11, even) = %d, want 12", got)
	}
	if got := findFreePort(11, func(p int) bool { return p == 10 }); got != 10 {
		t.Errorf("findFreePort(11, only 10 free) = %d, want 10", got)
	}
	if got := findFreePort(1, even); got != 2 {
		t.Errorf("findFreePort(1, even) = %d, want 2: below 1 is not a port", got)
	}
	if got := findFreePort(50, func(int) bool { return false }); got != 0 {
		t.Errorf("findFreePort(nothing free) = %d, want 0", got)
	}
	// 12 and 84 are free but both sit outside the window from 9000.
	if got := findFreePort(9000, func(p int) bool { return p == 12 || p == 84 }); got != 0 {
		t.Errorf("findFreePort(only far ports free) = %d, want 0", got)
	}
}

// TestHoldersText renders the holders for one message: the name when the
// system tools could say it, the pid always; two joined; three or more as
// the first two plus a count.
func TestHoldersText(t *testing.T) {
	if got := holdersText(nil); got != "" {
		t.Errorf("holdersText(nil) = %q, want empty", got)
	}
	if got := (procEntry{PID: "1"}).text(); got != "(pid 1)" {
		t.Errorf("nameless entry = %q, want (pid 1)", got)
	}
	if got := (procEntry{PID: "123", Name: "nginx"}).text(); got != "nginx (pid 123)" {
		t.Errorf("named entry = %q, want nginx (pid 123)", got)
	}
	two := holdersText([]procEntry{{PID: "1", Name: "a"}, {PID: "2", Name: "b"}})
	if two != "a (pid 1)、b (pid 2)" {
		t.Errorf("two holders = %q", two)
	}
	many := holdersText([]procEntry{{PID: "1", Name: "a"}, {PID: "2", Name: "b"}, {PID: "3", Name: "c"}})
	if many != "a (pid 1)、b (pid 2) 等 3 个进程" {
		t.Errorf("three holders = %q", many)
	}
}

// TestClash is the sharing rule the engine applies and the suggestion
// walk assumes: only web rules of one protocol share a port — their
// routes group under one listener — while stream rules of any protocol
// and web rules of two protocols all collide.
func TestClash(t *testing.T) {
	cases := []struct {
		a, b model.Protocol
		want bool
	}{
		{model.TCP, model.TCP, true},
		{model.UDP, model.UDP, true},
		{model.HTTP, model.HTTP, false},
		{model.HTTPS, model.HTTPS, false},
		{model.TCP, model.UDP, true},
		{model.UDP, model.TCP, true},
		{model.TCP, model.HTTP, true},
		{model.HTTP, model.TCP, true},
		{model.HTTP, model.HTTPS, true},
		{model.HTTPS, model.HTTP, true},
	}
	for _, c := range cases {
		if got := clash(c.a, c.b); got != c.want {
			t.Errorf("clash(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// cannedHolders answers the platform's listing tools with streams naming
// one process, pid and name, as sitting on port: lsof's -F records on
// Unix, netstat -ano plus tasklist's CSV on Windows. A conflict scenario
// then reads the same on both, whichever tool the machine asks for.
func cannedHolders(port int, pid, name string) func() {
	return withToolExecuter(func(ctx context.Context, tool string, args []string) ([]byte, error) {
		switch tool {
		case "lsof":
			return []byte("p" + pid + "\nc" + name + "\n"), nil
		case "netstat":
			return []byte(fmt.Sprintf(
				"  TCP    0.0.0.0:%d           0.0.0.0:0              LISTENING       %s\r\n",
				port, pid)), nil
		case "tasklist":
			return []byte(fmt.Sprintf("\"%s.exe\",\"%s\",\"Console\",\"1\",\"1,024 K\"\r\n", name, pid)), nil
		}
		return nil, errors.New("not this tool")
	})
}

// TestWhoHoldsDegraded checks the empty answers: a tool that is missing,
// one that prints nothing, and one that hangs past its deadline all leave
// the caller with no name, which the message renders as a bare "taken".
func TestWhoHoldsDegraded(t *testing.T) {
	t.Run("missing tool", func(t *testing.T) {
		t.Cleanup(withToolExecuter(func(ctx context.Context, name string, args []string) ([]byte, error) {
			return nil, exec.ErrNotFound
		}))
		if got := whoHolds(9000, "TCP"); got != nil {
			t.Fatalf("missing tool = %+v, want nil", got)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		t.Cleanup(withToolExecuter(func(ctx context.Context, name string, args []string) ([]byte, error) {
			return []byte("  \n"), nil
		}))
		if got := whoHolds(9000, "TCP"); got != nil {
			t.Fatalf("empty output = %+v, want nil", got)
		}
	})

	t.Run("past the deadline", func(t *testing.T) {
		t.Cleanup(withToolExecuter(func(ctx context.Context, name string, args []string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}))
		if got := whoHolds(9000, "TCP"); got != nil {
			t.Fatalf("timed-out tool = %+v, want nil", got)
		}
	})
}

// filledForm returns a complete stream form listening on port, the smallest
// thing SaveForm accepts.
func filledForm(port int) FormValues {
	f := NewForm()
	f.Name = "探测"
	f.ListenPort = strconv.Itoa(port)
	f.UpstreamHost = "127.0.0.1"
	f.UpstreamPort = "3306"
	return f
}

// TestSaveFormPortConflict walks the form's whole port-conflict path: the
// save is refused with the holder named and a free port offered, the fix
// fills the field, and the resave on the offered port goes through.
func TestSaveFormPortConflict(t *testing.T) {
	a := droppedApp(t)

	// Another saved rule, so the suggestion has a mapping port to step over.
	claimed := sparePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "已占映射", Protocol: model.TCP, Enabled: true,
		ListenPort: claimed, UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// A listener this process holds, so nothing can bind the port.
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(sparePort(t)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	held := ln.Addr().(*net.TCPAddr).Port

	t.Cleanup(cannedHolders(held, "321", "python3"))

	// A free port saves without a fix to offer.
	a.OpenForm(nil)
	a.SetForm(filledForm(sparePort(t)))
	if err := a.SaveForm(); err != nil {
		t.Fatalf("a free port should save: %v", err)
	}
	if fix := a.FormPortFix(); fix != 0 {
		t.Fatalf("a free port should offer no fix, got %d", fix)
	}

	// A taken port refuses, names the holder, and offers a nearby port.
	a.OpenForm(nil)
	a.SetForm(filledForm(held))
	if err := a.SaveForm(); err == nil {
		t.Fatal("saving onto a taken port should be refused")
	}
	_, _, _, errs := a.FormState()
	// The exact spelling of the name is the platform's (python3 here,
	// python3.exe there); the pid is what both can say.
	if msg := errs["ListenPort"]; !strings.Contains(msg, "占用") ||
		!strings.Contains(msg, "pid 321") ||
		!strings.Contains(msg, strconv.Itoa(held)) {
		t.Fatalf("ListenPort error = %q, want the port, the holder and the word 占用", msg)
	}
	fix := a.FormPortFix()
	if fix == 0 || fix == held || fix == claimed {
		t.Fatalf("the offered port = %d, want free, neither %d nor the mapping's %d", fix, held, claimed)
	}
	// 已占映射 and the first, saved rule are the only rows: the refused
	// save wrote nothing.
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 2 {
		t.Fatalf("the refused save must not write a row: %d rows", len(list))
	}

	// The fix fills the field, and the resave on it goes through.
	a.UseFormPortFix()
	_, _, f, _ := a.FormState()
	if f.ListenPort != strconv.Itoa(fix) {
		t.Fatalf("after the fix ListenPort = %q, want %d", f.ListenPort, fix)
	}
	if err := a.SaveForm(); err != nil {
		t.Fatalf("saving on the offered port: %v", err)
	}
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if len(list) != 3 || list[2].ListenPort != fix {
		t.Fatalf("the resaved rule should sit on %d: %+v", fix, list)
	}
}

// TestSaveFormPortClaimedByMapping checks the other way a port is taken:
// another rule already owns it, and the message says whose it is rather
// than naming a process — the fix is the same nearby walk.
func TestSaveFormPortClaimedByMapping(t *testing.T) {
	a := droppedApp(t)
	claimed := sparePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "数据库", Protocol: model.TCP, Enabled: true,
		ListenPort: claimed, UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// Hold the port for real too, so the probe has a reason to look.
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(claimed))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	a.OpenForm(nil)
	a.SetForm(filledForm(claimed))
	if err := a.SaveForm(); err == nil {
		t.Fatal("saving onto a claimed port should be refused")
	}
	_, _, _, errs := a.FormState()
	if msg := errs["ListenPort"]; !strings.Contains(msg, "「数据库」") {
		t.Fatalf("ListenPort error = %q, want it to name the claiming mapping", msg)
	}
	if fix := a.FormPortFix(); fix == 0 || fix == claimed {
		t.Fatalf("the offered port = %d, want one away from %d", fix, claimed)
	}
}

// TestBindHint checks the sentence added to a bind failure from apply or
// preflight: two rules on one port name the pair, a foreign holder is the
// process the tools found, and a silent machine still gets the suggestion.
func TestBindHint(t *testing.T) {
	a := droppedApp(t)
	held := sparePort(t)
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(held))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	t.Cleanup(cannedHolders(held, "77", "docker"))

	t.Run("a lone rule names the holder", func(t *testing.T) {
		err := &engine.ErrBind{Port: held, Proto: "TCP", Err: errors.New("address already in use")}
		hint := a.bindHint(err)
		for _, want := range []string{strconv.Itoa(held), "pid 77", "可改用"} {
			if !strings.Contains(hint, want) {
				t.Fatalf("hint = %q, want it to contain %q", hint, want)
			}
		}
	})

	t.Run("two rules name the pair", func(t *testing.T) {
		for _, name := range []string{"一", "二"} {
			if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
				Name: name, Protocol: model.TCP, Enabled: true,
				ListenPort: held, UpstreamHost: "127.0.0.1", UpstreamPort: 1,
			}); err != nil {
				t.Fatal(err)
			}
		}
		hint := a.bindHint(&engine.ErrBind{Port: held, Proto: "TCP", Err: errors.New("address already in use")})
		if !strings.Contains(hint, "「一」和「二」") {
			t.Fatalf("hint = %q, want the pair named", hint)
		}
	})
}
