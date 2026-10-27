package e2e

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
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

	// The draft opens in the form; saving it is allowed — the conflict is
	// the engine's to report on apply — but the row says why it is not
	// listening, so the user is not left guessing.
	a.OpenNewFormFrom(*draft)
	if err := a.SaveForm(); err != nil {
		t.Fatalf("saving a conflicted draft should be allowed: %v", err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	if listening, problem := a.RuleState(draft.ID); listening || problem == "" {
		t.Errorf("the rule should say why it is not listening: %q", problem)
	}
}

// itoa renders a port for the fake reply.
func itoa(n int) string { return strconv.Itoa(n) }

// TestAIProbeAnswers checks the settings page's 测试连接 path: the probe
// costs one small request, and what the endpoint answers decides the
// outcome the user sees.
func TestAIProbeAnswers(t *testing.T) {
	t.Run("a live endpoint answers", func(t *testing.T) {
		a := testApp(t)
		srv := fakeOpenAI(t, `{"name":"x","protocol":"tcp","upstreamHost":"10.0.0.1",
			"upstreamPort":3306,"listenPort":9000,"explanation":"ok"}`)
		a.SetAI(srv.URL, "test-model")
		if err := a.ProbeAI(); err != nil {
			t.Errorf("a reachable endpoint should probe clean: %v", err)
		}
	})
	t.Run("a refused endpoint says so", func(t *testing.T) {
		a := testApp(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(srv.Close)
		a.SetAI(srv.URL, "test-model")
		err := a.ProbeAI()
		if err == nil {
			t.Fatal("a 401 endpoint should fail the probe")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("the error should name the status: %v", err)
		}
	})
	t.Run("an unconfigured endpoint is refused before the request", func(t *testing.T) {
		a := testApp(t)
		if err := a.ProbeAI(); err == nil {
			t.Fatal("probing without settings should fail")
		} else if !strings.Contains(err.Error(), "地址") {
			t.Errorf("the error should say what to fill: %v", err)
		}
	})
}

// TestAITestConnectionButton walks the button itself: the click hands the
// probe to the background, the app stays responsive, and a failure lands
// in the status line rather than a crash.
func TestAITestConnectionButton(t *testing.T) {
	a := testApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 900)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The AI card sits below the fold of the window's default height; the
	// test scrolls to it, as a user would.
	clickScrolled(t, tt, "测试连接")
	// The probe runs in the background; the failure arrives as a status.
	waitFor(t, tt, "the probe result", func() bool {
		st, ok := a.Status()
		return ok && st.IsErr && strings.Contains(st.Text, "500")
	})
}

// fakeModels is an endpoint that answers GET /v1/models, so the 拉取模型
// path runs end to end without a network.
func fakeModels(t *testing.T, models ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/models") {
			http.NotFound(w, r)
			return
		}
		data := make([]any, 0, len(models))
		for _, m := range models {
			data = append(data, map[string]any{"id": m})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAIListModels walks 拉取模型: configuring an endpoint, clicking the
// button, and the built-in draft flow still working against the fetched
// model. It checks the UI reaches the list rather than that a real gateway
// answers.
func TestAIListModels(t *testing.T) {
	a := testApp(t)
	srv := fakeModels(t, "deepseek-chat", "gpt-4o-mini")
	tt := tester(t, a, 1200, 900)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillFieldScrolled(t, tt, "接口地址", srv.URL)
	// The AI card sits below the fold of the window's default height.
	clickScrolled(t, tt, "拉取模型")
	// The fetch runs in a goroutine; the model list lands and the card says so.
	waitFor(t, tt, "the model list", func() bool {
		return tt.HasText("已拉取 2 个模型")
	})
	// The fetch suggests the first sorted model into the field, so the
	// user who only wanted a working default still has one step: 保存.
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.AIReady() {
		t.Fatalf("saving a fetched model should make the AI ready: model=%q", a.AIModel())
	}
	if a.AIModel() != "deepseek-chat" {
		t.Errorf("the suggestion should be the first sorted model: %q", a.AIModel())
	}

	// Choosing the second model from the picker changes what is saved.
	reveal(t, tt, "从列表选")
	if err := tt.Click("从列表选"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The picker's rows carry a 选-prefix label so a screen reader (and
	// this test) can tell them from the field's own text.
	if err := tt.Click("选 gpt-4o-mini"); err != nil {
		t.Fatalf("the picker should list the fetched models: %v", err)
	}
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.AIModel() != "gpt-4o-mini" {
		t.Errorf("choosing from the list should save the choice: %q", a.AIModel())
	}

	// A gateway whose list omits a model still accepts its name typed by
	// hand: the field is the truth, the list only fills it in.
	fillField(t, tt, "模型", "my-private-model")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.AIModel() != "my-private-model" {
		t.Errorf("a hand-typed model must not be discarded: %q", a.AIModel())
	}
}

// TestAIListModelsReportsFailure checks the failure path: a gateway that
// refuses the list shows the reason, and no model picker appears.
func TestAIListModelsReportsFailure(t *testing.T) {
	a := testApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "bad key"},
		})
	}))
	t.Cleanup(srv.Close)

	tt := tester(t, a, 1200, 900)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillFieldScrolled(t, tt, "接口地址", srv.URL)
	// The settings page has grown past the window's default height; the AI
	// card sits below the fold until the test scrolls to it.
	clickScrolled(t, tt, "拉取模型")
	waitFor(t, tt, "the failure", func() bool { return tt.HasText("拉取失败") })
	if !tt.HasText("401") {
		t.Errorf("the failure should name the status: %v", tt.Texts())
	}
}
