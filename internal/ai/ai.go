// Package ai turns a sentence into a mapping draft, with an OpenAI
// compatible endpoint.
//
// The model only ever produces a draft. Nothing here writes a
// configuration, touches nginx or stores a mapping: the caller shows the
// draft in the form, the user fixes what is wrong, and saving goes through
// the same validation a hand-typed mapping does.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// normalizeBase trims a user-typed API root so callers can append /v1/...
// once.
//
// Gateways are advertised both as http://host:3425 and as
// http://host:3425/v1. Models already stripped a trailing /v1; Draft used
// to append /v1/chat/completions onto either form, so a base that already
// carried /v1 became /v1/v1/chat/completions and 404'd. Every request goes
// through this first: trim space and trailing slashes, then peel trailing
// /v1 segments (repeatedly, so a pasted .../v1/v1 works too). Only whole
// segments go: "https://host/api/v1" keeps "api" and loses just the /v1.
func normalizeBase(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	for strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = strings.TrimRight(base[:len(base)-3], "/")
	}
	return base
}

// Settings configure the client.
type Settings struct {
	// BaseURL is the API root, such as https://api.openai.com or a local
	// gateway's address, with or without a trailing /v1. Requests append
	// /v1/... after normalizeBase.
	BaseURL string
	// Model is the model name the gateway knows.
	Model string
	// APIKey is read from the system credential store, never from a file
	// the app exports.
	APIKey string
	// Timeout bounds one request.
	Timeout time.Duration
}

// Ready reports whether the settings are enough to make a request.
func (s Settings) Ready() bool {
	return strings.TrimSpace(s.BaseURL) != "" && strings.TrimSpace(s.Model) != ""
}

// systemPrompt tells the model what it is doing and what the fields mean.
const systemPrompt = `You turn a Chinese or English sentence into one nginx forwarding rule.

The app forwards a port on this machine to a service on the internal
network. Input may be Chinese ("把内网 MySQL 暴露到 9000") or English.

Rules:
- protocol is "tcp" for databases, SSH, RDP and raw ports, "udp" for DNS and
  other datagram protocols, "http" or "https" only for web services.
- listenPort is the port on THIS machine; upstreamPort is the service's.
  If the user does not say a listen port, choose a sensible one above 8000.
- Never invent an IP address the user did not give. If the upstream is
  missing, use "" and say so in the explanation.
- extraDirectives: only plain directives such as "proxy_timeout 1h;".
  Never write a server block, a brace, or anything that could contain one.
- explanation says, in Chinese, what you understood and what is missing.

Reply with the JSON object only.`

// Draft is what the model proposes.
//
// The fields a model is free to misshape are json.RawMessage and decoded by
// UnmarshalJSON, which tolerates a port quoted as a string, a directive
// list, or an enum alias — and drops one broken field instead of failing
// the whole draft. Only a reply that is not a JSON object at all is
// refused, which keeps the "model returned something else" signal.
type Draft struct {
	Name            string
	Protocol        string
	ListenPort      int
	ServerNames     string
	ListenPath      string
	UpstreamHost    string
	UpstreamPort    int
	ProxyMode       string
	Group           string
	ExtraDirectives string
	Explanation     string
}

// Mapping converts the draft, through the same validation the form uses.
//
// A draft is untrusted input: the model may have hallucinated a port, an
// address, or a directive. Everything except the explanation is run
// through the app's sanitizers, and a field that fails is dropped rather
// than passed on. A proxy mode the app does not know falls back to the
// standard one, and a group the schema has no field for is ignored; both
// are said so in the explanation, so what the form shows never quietly
// differs from what the model said.
func (d Draft) Mapping() (model.Mapping, string, error) {
	proto := model.Protocol(strings.ToLower(strings.TrimSpace(d.Protocol)))
	if !proto.Valid() {
		return model.Mapping{}, "", fmt.Errorf("模型给出的协议 %q 不受支持", d.Protocol)
	}
	mode, modeNote := coerceProxyMode(d.ProxyMode)
	explanation := joinNotes(d.Explanation, modeNote, dropGroup(d.Group))
	m := model.Mapping{
		Name:            strings.TrimSpace(d.Name),
		Protocol:        proto,
		Enabled:         true,
		ListenPort:      d.ListenPort,
		ServerNames:     model.StripInject(d.ServerNames),
		ListenPath:      model.StripInject(d.ListenPath),
		UpstreamHost:    model.StripInject(d.UpstreamHost),
		UpstreamPort:    d.UpstreamPort,
		ProxyMode:       mode,
		ExtraDirectives: sanitizeDirectives(d.ExtraDirectives),
	}
	if m.Name == "" {
		m.Name = "AI 映射"
	}
	if m.ListenPath == "" {
		m.ListenPath = "/"
	}
	if m.ListenPort != 0 && !model.ValidPort(m.ListenPort) {
		m.ListenPort = 0 // let the user choose
	}
	if !model.ValidPort(m.UpstreamPort) {
		return model.Mapping{}, explanation, fmt.Errorf("模型没有给出可用的上游端口")
	}
	if m.UpstreamHost == "" {
		return model.Mapping{}, explanation, fmt.Errorf("模型没有给出上游地址，请在表单里补上")
	}
	return m, explanation, nil
}

// sanitizeDirectives keeps extra directives to plain `name value;` lines.
//
// The field accepts raw nginx by design, but never from a model: a line
// that could open or close a block, or define another server, is dropped.
func sanitizeDirectives(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.ContainsAny(t, "{}#") {
			continue
		}
		if !strings.HasSuffix(t, ";") {
			continue
		}
		out = append(out, t)
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return strings.Join(out, "\n")
}

// fieldAliases lists the names a model writes for one draft field. The
// prompt asks for camelCase, and live models keep drifting: one gateway's
// small model answers upstreamHost as "upstreamIp". The first alias that
// arrives non-empty wins.
var fieldAliases = map[string][]string{
	"name":            {"name"},
	"protocol":        {"protocol"},
	"listenPort":      {"listenPort", "listen_port", "listen", "port"},
	"serverNames":     {"serverNames", "server_names", "serverName"},
	"listenPath":      {"listenPath", "listen_path", "path"},
	"upstreamHost":    {"upstreamHost", "upstream_host", "upstreamIp", "upstream_ip", "upstreamAddr", "host", "target"},
	"upstreamPort":    {"upstreamPort", "upstream_port", "targetPort", "target_port"},
	"proxyMode":       {"proxyMode", "proxy_mode"},
	"group":           {"group"},
	"extraDirectives": {"extraDirectives", "extra_directives", "directives"},
	"explanation":     {"explanation", "note"},
}

// UnmarshalJSON decodes a model's reply into a Draft, leniently.
//
// The reply is a JSON object, but the model decides the shape of each
// field: a port may arrive as 9000 or "9000", extraDirectives as a string
// or a list, a field under one of its alias names, and enum values in any
// casing or alias. Rather than let one odd field void the whole draft,
// every field is decoded on its own and a failure leaves that field at
// its zero value. A reply that is not an object at all stays an error:
// that is the "model did not answer with JSON" case the caller reports.
func (d *Draft) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	str := func(aliases ...string) string {
		for _, a := range aliases {
			if raw, ok := fields[a]; ok {
				if v := flexString(raw); v != "" {
					return v
				}
			}
		}
		return ""
	}
	port := func(aliases ...string) int {
		for _, a := range aliases {
			if raw, ok := fields[a]; ok {
				if v := flexPort(raw); v != 0 {
					return v
				}
			}
		}
		return 0
	}
	d.Name = str(fieldAliases["name"]...)
	d.Protocol = str(fieldAliases["protocol"]...)
	d.ListenPort = port(fieldAliases["listenPort"]...)
	d.ServerNames = str(fieldAliases["serverNames"]...)
	d.ListenPath = str(fieldAliases["listenPath"]...)
	d.UpstreamHost = str(fieldAliases["upstreamHost"]...)
	d.UpstreamPort = port(fieldAliases["upstreamPort"]...)
	d.ProxyMode = str(fieldAliases["proxyMode"]...)
	d.Group = str(fieldAliases["group"]...)
	d.ExtraDirectives = str(fieldAliases["extraDirectives"]...)
	d.Explanation = str(fieldAliases["explanation"]...)
	return nil
}

// flexString reads a field that may be a string, or a list of strings.
//
// A list is joined with newlines, which is what sanitizeDirectives splits
// on again; entries that are not strings (a number, a nested object) are
// dropped rather than joined into nonsense.
func flexString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return ""
	}
	var out []string
	for _, item := range list {
		if v := flexString(item); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, "\n")
}

// flexPort reads a port that may be a number or a numeric string. Anything
// else reads as 0, which the existing validation already handles.
func flexPort(raw json.RawMessage) int {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		v, err := strconv.Atoi(strings.TrimSpace(str))
		if err == nil {
			return v
		}
	}
	return 0
}

// proxyModeAliases maps the names a model tends to invent to the modes the
// app has. An alias that could mean two things is not listed: an unknown
// value falls back to the standard mode with a note, instead of a guess
// the user cannot see.
var proxyModeAliases = map[string]model.ProxyMode{
	"standard":    model.ProxyStandard,
	"std":         model.ProxyStandard,
	"default":     model.ProxyStandard,
	"normal":      model.ProxyStandard,
	"full":        model.ProxyStandard,
	"minimal":     model.ProxyMinimal,
	"min":         model.ProxyMinimal,
	"fast":        model.ProxyMinimal,
	"simple":      model.ProxyMinimal,
	"passthrough": model.ProxyMinimal,
	"custom":      model.ProxyCustom,
	"none":        model.ProxyCustom,
}

// coerceProxyMode maps a draft's proxy mode to a valid one, and says how.
func coerceProxyMode(s string) (model.ProxyMode, string) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return model.ProxyStandard, ""
	}
	if mode, ok := proxyModeAliases[v]; ok {
		if mode != model.ProxyMode(v) {
			return mode, fmt.Sprintf("代理模式 %q 已按 %s 处理", v, mode.Label())
		}
		return mode, ""
	}
	return model.ProxyStandard, fmt.Sprintf("代理模式 %q 不受支持，已按 标准反代 处理", v)
}

// dropGroup explains that a group field was ignored: the app's schema has
// no such grouping, so the value goes nowhere rather than into a field
// that would silently drop it later.
func dropGroup(group string) string {
	if strings.TrimSpace(group) == "" {
		return ""
	}
	return fmt.Sprintf("已忽略不支持的分组字段 %q", group)
}

// joinNotes appends the per-field notes to the model's explanation, so the
// user reads what was adjusted in the same place as the draft itself.
func joinNotes(explanation string, notes ...string) string {
	var kept []string
	if e := strings.TrimSpace(explanation); e != "" {
		kept = append(kept, e)
	}
	for _, n := range notes {
		if n != "" {
			kept = append(kept, n)
		}
	}
	return strings.Join(kept, "\n")
}

// Client calls an OpenAI compatible endpoint.
type Client struct {
	settings Settings
	http     *http.Client
}

// New returns a client.
func New(s Settings) *Client {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{settings: s, http: &http.Client{
		Timeout:       timeout,
		CheckRedirect: sameOriginOnly,
	}}
}

// sameOriginOnly keeps the API key from following a redirect off the
// endpoint the user configured.
//
// Go already drops Authorization across hosts, but not across a port or
// scheme on the same host: a gateway that answers 302 to its own other
// port, or to plain http, would receive the bearer token. A forwarding
// endpoint never needs to redirect a POST, so refusing is safe.
func sameOriginOnly(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	first := via[0].URL
	if req.URL.Host != first.Host || req.URL.Scheme != first.Scheme {
		return errors.New("AI 服务重定向到别的地址，已停止发送凭据")
	}
	if len(via) >= 3 {
		return errors.New("AI 服务重定向次数过多")
	}
	return nil
}

// request and response shapes, narrowed to what the client reads.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Draft asks the model for a mapping from a sentence.
func (c *Client) Draft(ctx context.Context, sentence string) (Draft, error) {
	if !c.settings.Ready() {
		return Draft{}, errors.New("还没有配置 AI 服务")
	}
	body, err := json.Marshal(chatRequest{
		Model:       c.settings.Model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: sentence},
		},
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
	})
	if err != nil {
		return Draft{}, err
	}

	url := normalizeBase(c.settings.BaseURL) + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Draft{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.settings.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Draft{}, fmt.Errorf("连接 AI 服务失败：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Draft{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var r chatResponse
		if json.Unmarshal(data, &r) == nil && r.Error != nil && r.Error.Message != "" {
			return Draft{}, fmt.Errorf("AI 服务返回 %d：%s", resp.StatusCode, r.Error.Message)
		}
		return Draft{}, fmt.Errorf("AI 服务返回 %d", resp.StatusCode)
	}

	var r chatResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return Draft{}, fmt.Errorf("无法解析 AI 服务的响应：%w", err)
	}
	if len(r.Choices) == 0 {
		return Draft{}, errors.New("AI 服务没有返回内容")
	}
	content := extractJSON(r.Choices[0].Message.Content)

	var d Draft
	if err := json.Unmarshal([]byte(content), &d); err != nil {
		return Draft{}, fmt.Errorf("模型返回的不是预期的 JSON：%w", err)
	}
	return d, nil
}

// extractJSON pulls the object out of a reply that may still wrap it in a
// code fence, which some gateways do despite the format request.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// Probe checks that the endpoint answers, for the settings page's test
// button. It costs one small request.
func (c *Client) Probe(ctx context.Context) error {
	if !c.settings.Ready() {
		return errors.New("请先填写地址和模型")
	}
	_, err := c.Draft(ctx, "把内网 MySQL 3306 暴露到 9000")
	return err
}

// modelsResponse is the OpenAI shape of GET /v1/models, narrowed to the
// one field worth reading: each entry's id, which is what a request names.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Models lists what the endpoint offers, for the settings page's picker.
//
// The list comes from GET /v1/models, which every OpenAI compatible
// gateway answers. The base goes through normalizeBase so an address typed
// with or without /v1, trailing slashes and all, reaches the same path.
// The ids are sorted and de-duplicated: gateways repeat them, and a
// dropdown of "gpt-4o, gpt-4o, gpt-4o" reads as a bug.
func (c *Client) Models(ctx context.Context) ([]string, error) {
	base := normalizeBase(c.settings.BaseURL)
	if base == "" {
		return nil, errors.New("请先填写接口地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if c.settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.settings.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 AI 服务失败：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var mr modelsResponse
		msg := ""
		if json.Unmarshal(data, &mr) == nil && mr.Error != nil {
			msg = "：" + mr.Error.Message
		}
		return nil, fmt.Errorf("AI 服务返回 %d%s", resp.StatusCode, msg)
	}
	var mr modelsResponse
	if err := json.Unmarshal(data, &mr); err != nil {
		return nil, fmt.Errorf("无法解析模型列表：%w", err)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(mr.Data))
	for _, m := range mr.Data {
		if m.ID != "" && !seen[m.ID] {
			seen[m.ID] = true
			out = append(out, m.ID)
		}
	}
	sort.Strings(out)
	return out, nil
}
