package e2e

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The port-conflict path, walked the way a user takes it: the form names
// who sits on a taken port and offers the nearest free one, one click
// moves the rule there, and a failed apply says the same things under the
// button. The occupier throughout is the test itself, holding real
// listeners the way a foreign program would.

// TestFormPortConflictOffersFix saves a rule onto a port the machine
// already uses: the form refuses, names the holder, and the 改用 button
// moves the rule to the offered port, where the save goes through.
func TestFormPortConflictOffersFix(t *testing.T) {
	a := testApp(t)
	held := holdPort(t)

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "冲突验证")
	fillField(t, tt, "监听端口", strconv.Itoa(held))
	fillField(t, tt, "上游主机", "127.0.0.1")
	fillField(t, tt, "上游端口", "3306")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}

	// The form stays open with the holder named and a free port offered.
	waitFor(t, tt, "the conflict named", func() bool {
		_, _, _, errs := a.FormState()
		return errs["ListenPort"] != ""
	})
	// A settled frame: the one waitFor stopped on may predate the error
	// landing, and the fix row is what the next frame draws.
	tt.Frame()
	fix := a.FormPortFix()
	if fix == 0 || fix == held {
		t.Fatalf("a free port should be offered beside %d, got %d", held, fix)
	}
	_, _, _, errs := a.FormState()
	if msg := errs["ListenPort"]; msg == "" ||
		!strings.Contains(msg, strconv.Itoa(held)) || !strings.Contains(msg, "占用") {
		t.Fatalf("the error should name the port and its holder: %q", msg)
	}
	// The machine this test runs on can name the holder (lsof); one that
	// cannot still says the port is taken, which is the floor, not a
	// failure.
	if !tt.HasText("或改用") {
		t.Fatalf("the form should offer the fix: %v", tt.Texts())
	}

	// One click moves the rule; the error it answered is retired with it.
	label := fmt.Sprintf("改用 %d", fix)
	reveal(t, tt, label)
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking the fix: %v\nshown: %v", err, tt.Texts())
	}
	waitFor(t, tt, "the fix in the field", func() bool {
		_, _, f, errs := a.FormState()
		return f.ListenPort == strconv.Itoa(fix) && errs["ListenPort"] == ""
	})
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the rule saved on the offered port", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 1 && list[0].ListenPort == fix
	})
}

// TestApplyFailureNamesHolder checks the bar under the apply button: a
// port another program holds fails the apply with the holder named and a
// free port offered in the same sentence.
func TestApplyFailureNamesHolder(t *testing.T) {
	a := testApp(t)
	held := holdPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "占用演示", Protocol: model.TCP, Enabled: true, ListenPort: held,
		UpstreamHost: "127.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the failure bar", func() bool { return tt.HasText("无法监听") })
	for _, want := range []string{strconv.Itoa(held), "占用", "可改用"} {
		if !tt.HasText(want) {
			t.Errorf("the bar should say %q: %v", want, tt.Texts())
		}
	}
	if fix := a.FormPortFix(); fix != 0 {
		t.Errorf("an apply failure offers no form fix, got %d", fix)
	}
}
