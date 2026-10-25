package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeLLM answers like an OpenAI compatible endpoint, without a network.
func fakeLLM(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv
}

func chatReply(content string) []byte {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	return b
}

func TestDraftParsesReply(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		// Some gateways wrap the JSON in a code fence despite the request.
		w.Write(chatReply("```json\n" + `{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,"upstreamHost":"192.168.1.10","upstreamPort":3306,"proxyMode":"standard","extraDirectives":"proxy_timeout 1h;","explanation":"把内网 MySQL 暴露到 9000"}` + "\n```"))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test", APIKey: "sk-test"})
	d, err := c.Draft(context.Background(), "把内网 mysql 暴露到 9000")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "内网 MySQL" || d.Protocol != "tcp" || d.ListenPort != 9000 {
		t.Fatalf("draft: %+v", d)
	}
	m, explanation, err := d.Mapping()
	if err != nil {
		t.Fatal(err)
	}
	if m.UpstreamHost != "192.168.1.10" || m.UpstreamPort != 3306 || m.ExtraDirectives != "proxy_timeout 1h;" {
		t.Errorf("mapping: %+v", m)
	}
	if explanation == "" {
		t.Error("the explanation should be kept")
	}
}

func TestDraftDropsInjectedDirectives(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		// The JSON has real newlines escaped inside the string value.
		w.Write(chatReply("{\"name\":\"x\",\"protocol\":\"tcp\",\"upstreamHost\":\"1.2.3.4\",\"upstreamPort\":80," +
			"\"extraDirectives\":\"proxy_timeout 1h;\\n} server { listen 6666;\\naccess_log /etc/passwd;\"}"))
	})
	c := New(Settings{BaseURL: srv.URL, Model: "test"})
	d, err := c.Draft(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	// The raw reply still holds what the model wrote; the sanitizing
	// happens when the draft becomes a mapping, which is what the form
	// saves.
	m, _, err := d.Mapping()
	if err != nil {
		t.Fatal(err)
	}
	if m.ExtraDirectives != "proxy_timeout 1h;\naccess_log /etc/passwd;" {
		t.Errorf("the line with braces should be dropped: %q", m.ExtraDirectives)
	}
	if strings.ContainsAny(m.ExtraDirectives, "{}") {
		t.Errorf("braces survived: %q", m.ExtraDirectives)
	}
}

func TestDraftRejectsHallucinatedValues(t *testing.T) {
	cases := []struct {
		name  string
		reply string
	}{
		{"bad protocol", `{"name":"x","protocol":"icmp","upstreamHost":"1.2.3.4","upstreamPort":80}`},
		{"no upstream", `{"name":"x","protocol":"tcp","upstreamHost":"","upstreamPort":80}`},
		{"no port", `{"name":"x","protocol":"tcp","upstreamHost":"1.2.3.4","upstreamPort":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) { w.Write(chatReply(tc.reply)) })
			c := New(Settings{BaseURL: srv.URL, Model: "t"})
			d, err := c.Draft(context.Background(), "x")
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := d.Mapping(); err == nil {
				t.Errorf("%s should be refused", tc.name)
			}
		})
	}
}

func TestDraftReportsHTTPError(t *testing.T) {
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "invalid api key"},
		})
	})
	c := New(Settings{BaseURL: srv.URL, Model: "t"})
	if _, err := c.Draft(context.Background(), "x"); err == nil {
		t.Fatal("a 401 should be an error")
	}
}

func TestDraftNeedsSettings(t *testing.T) {
	c := New(Settings{})
	if _, err := c.Draft(context.Background(), "x"); err == nil {
		t.Fatal("an unconfigured client should refuse")
	}
	if (Settings{}).Ready() {
		t.Error("empty settings are not ready")
	}
}

func TestExtractJSON(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"here it is: {\"a\":1} thanks", `{"a":1}`},
	} {
		if got := extractJSON(c.in); got != c.want {
			t.Errorf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestModelsListsEndpoint checks 拉取模型: the client reads GET /v1/models,
// sorts the ids, drops the duplicates a gateway repeats, and trims a base
// URL that already carries /v1 so the path is not doubled.
func TestModelsListsEndpoint(t *testing.T) {
	var gotPath string
	srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{"id": "gpt-4o-mini"},
				map[string]any{"id": "gpt-4o"},
				map[string]any{"id": "gpt-4o"},
				map[string]any{"id": ""},
				map[string]any{"id": "deepseek-chat"},
			},
		})
	})

	c := New(Settings{BaseURL: srv.URL + "/v1", APIKey: "sk-test"})
	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Errorf("path = %q, want /v1/models", gotPath)
	}
	want := []string{"deepseek-chat", "gpt-4o", "gpt-4o-mini"}
	if len(models) != len(want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Errorf("models[%d] = %q, want %q", i, models[i], want[i])
		}
	}
}

// TestModelsReportsErrors checks that a gateway refusing the list, or
// answering rubbish, is reported rather than shown as an empty picker.
func TestModelsReportsErrors(t *testing.T) {
	t.Run("a refused endpoint", func(t *testing.T) {
		srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "invalid api key"},
			})
		})
		c := New(Settings{BaseURL: srv.URL})
		_, err := c.Models(context.Background())
		if err == nil {
			t.Fatal("a 401 should be an error")
		}
		if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
			t.Errorf("the error should name the status and reason: %v", err)
		}
	})
	t.Run("no base url", func(t *testing.T) {
		c := New(Settings{})
		if _, err := c.Models(context.Background()); err == nil {
			t.Fatal("an empty address should be refused")
		}
	})
	t.Run("rubbish body", func(t *testing.T) {
		srv := fakeLLM(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		})
		c := New(Settings{BaseURL: srv.URL})
		if _, err := c.Models(context.Background()); err == nil {
			t.Fatal("a body that is not JSON should be an error")
		}
	})
}
