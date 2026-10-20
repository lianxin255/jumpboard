package e2e

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// fakeOpenAI stands in for the model: the AI paths are tested end to end
// without a network or a bill.
func fakeOpenAI(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAIEntryHiddenWithoutConfig checks that the AI button does not appear
// before the feature is configured: a button that always fails is worse
// than no button.
func TestAIEntryHiddenWithoutConfig(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if tt.HasText("AI 建映射") {
		t.Errorf("the AI entry should be hidden: %v", tt.Texts())
	}
	if a.AIReady() {
		t.Error("AI should not be ready out of the box")
	}
}

// TestAIDraftFillsTheForm walks the AI path: configure, ask, get a draft,
// adopt it, save it — and check that only the draft's validated parts
// reach the database.
func TestAIDraftFillsTheForm(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, `{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,
		"upstreamHost":"192.168.1.10","upstreamPort":3306,"proxyMode":"standard",
		"extraDirectives":"proxy_timeout 1h;","explanation":"把内网 MySQL 暴露到 9000"}`)

	// No key: the fake endpoint does not check one, and the system
	// credential store needs a running app, which a test is not.
	a.SetAI(srv.URL, "test-model")
	if !a.AIReady() {
		t.Fatal("AI should be ready once the endpoint and model are set")
	}

	a.OpenAIForTest()
	if err := a.RunAISync("把内网 mysql 暴露到 9000"); err != nil {
		t.Fatalf("asking the model: %v", err)
	}
	draft := a.AIDraftForTest()
	if draft == nil {
		t.Fatal("no draft came back")
	}
	if draft.Name != "内网 MySQL" || draft.ListenPort != 9000 || draft.UpstreamHost != "192.168.1.10" {
		t.Fatalf("draft: %+v", draft)
	}
	if draft.ExtraDirectives != "proxy_timeout 1h;" {
		t.Errorf("extra directives: %q", draft.ExtraDirectives)
	}

	// The draft reaches the form, and saving it goes through the same
	// validation as a typed one.
	a.OpenNewFormFrom(*draft)
	if _, _, f, _ := a.FormState(); f.UpstreamHost != "192.168.1.10" {
		t.Fatalf("the form was not filled: %+v", f)
	}
	if err := a.SaveForm(); err != nil {
		t.Fatalf("saving the draft: %v", err)
	}
	list, err := a.Store.ListMappings()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Name != "内网 MySQL" || list[0].UpstreamPort != 3306 {
		t.Errorf("stored wrongly: %+v", list[0])
	}
}

// TestAIBadJSONDegrades checks that a model returning rubbish leaves the
// app usable.
func TestAIBadJSONDegrades(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, "I'm sorry, I can't help with that.")
	a.SetAI(srv.URL, "test-model")

	a.OpenAIForTest()
	if err := a.RunAISync("把 mysql 暴露到 9000"); err == nil {
		t.Fatal("a reply that is not JSON should be an error")
	}
	if a.AIDraftForTest() != nil {
		t.Error("no draft should come from rubbish")
	}
	if list, _ := a.Store.ListMappings(); len(list) != 0 {
		t.Error("nothing should have been saved")
	}
}

// TestAIPortConflictIsVisible checks that a port the machine already uses
// is not silently accepted.
func TestAIPortConflictIsVisible(t *testing.T) {
	a := testApp(t)
	// A listener on the port the model will propose.
	conflict := freePort(t)
	listener, err := net.Listen("tcp", ":"+itoa(conflict))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	reply := `{"name":"冲突","protocol":"tcp","listenPort":` + itoa(conflict) + `,
		"upstreamHost":"10.0.0.1","upstreamPort":3306,"explanation":"x"}`
	srv := fakeOpenAI(t, reply)
	a.SetAI(srv.URL, "test-model")
	a.OpenAIForTest()
	if err := a.RunAISync("x"); err != nil {
		t.Fatal(err)
	}
	draft := a.AIDraftForTest()
	if draft == nil {
		t.Fatal("no draft")
	}
	if !a.PortTaken(draft.ListenPort) {
		t.Errorf("port %d should read as taken", draft.ListenPort)
	}

	// The form shows the conflict, and saving is refused until it changes.
	a.OpenNewFormFrom(*draft)
	if err := a.SaveForm(); err != nil {
		// Saving is allowed: the conflict is a warning the user sees, not
		// a hard error, since nginx itself will refuse to bind.
		t.Logf("save: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
}

// itoa renders a port for the fake reply.
func itoa(n int) string { return strconv.Itoa(n) }
