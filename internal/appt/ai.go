package appt

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// aiSecretKey is the credential-store entry the API key lives in: the
// system's keychain on macOS, the Credential Manager on Windows. It never
// goes into the database or an export.
const aiSecretKey = "ai-api-key"

// AIBaseURL returns the configured endpoint.
func (a *App) AIBaseURL() string {
	return a.Store.Setting(store.SettingAIBaseURL, "")
}

// AIModel returns the configured model name.
func (a *App) AIModel() string {
	return a.Store.Setting(store.SettingAIModel, "")
}

// SetAI stores the endpoint and model. The key is separate, see SetAIKey.
func (a *App) SetAI(baseURL, model string) error {
	baseURL, model = strings.TrimSpace(baseURL), strings.TrimSpace(model)
	if err := a.Store.SetSetting(store.SettingAIBaseURL, baseURL); err != nil {
		return err
	}
	if err := a.Store.SetSetting(store.SettingAIModel, model); err != nil {
		return err
	}
	a.notify()
	return nil
}

// AIDiagModel returns the model the failure diagnosis and the
// interpretations use, "" when they use the main one.
func (a *App) AIDiagModel() string {
	return strings.TrimSpace(a.Store.Setting(store.SettingAIDiagModel, ""))
}

// SetAIDiagModel stores the diagnosis model. Empty means the main model,
// which is what the settings page's placeholder says.
func (a *App) SetAIDiagModel(m string) error {
	if err := a.Store.SetSetting(store.SettingAIDiagModel, strings.TrimSpace(m)); err != nil {
		return err
	}
	a.notify()
	return nil
}

// AIReady reports whether the AI panel should show at all: without an
// endpoint and a model the feature stays hidden rather than offering a
// button that fails.
func (a *App) AIReady() bool {
	return a.AIBaseURL() != "" && a.AIModel() != ""
}

// HasAIKey reports whether a key is stored.
func (a *App) HasAIKey() bool {
	v, err := readSecret(aiSecretKey)
	return err == nil && v != ""
}

// noKeychain reports whether the process must skip the system credential
// store (tests, headless CI). JUMPBOARD_NO_KEYCHAIN is preferred;
// NGJ_NO_KEYCHAIN remains for older scripts.
func noKeychain() bool {
	if os.Getenv("JUMPBOARD_NO_KEYCHAIN") == "1" {
		return true
	}
	return os.Getenv("NGJ_NO_KEYCHAIN") == "1"
}

// SetAIKey stores the API key in the system credential store.
//
// The store is written from its own goroutine with a deadline, like the
// read: a locked keychain would otherwise freeze the window.
func (a *App) SetAIKey(key string) error {
	if noKeychain() {
		return fmt.Errorf("当前环境没有系统凭据库，无法保存 API Key")
	}
	// The goroutine below outlives a timed-out caller: the credential
	// store is a C binding with no cancellation, so it stays blocked on
	// the keychain until the system answers. One stranded goroutine per
	// timed-out call is the price of never blocking the UI thread on it.
	ch := make(chan error, 1)
	key = strings.TrimSpace(key)
	go func() {
		if key == "" {
			ch <- mygo.Secrets.Delete(aiSecretKey)
			return
		}
		ch <- mygo.Secrets.Set(aiSecretKey, []byte(key))
	}()
	select {
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("保存 API Key 失败：%w", err)
		}
		a.notify()
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("保存 API Key 超时，请检查系统凭据库是否已解锁")
	}
}

// readSecret returns a secret, giving up after a moment.
//
// The system's credential store can block while it asks the user to unlock
// it, and on a machine without a session it never answers at all. A request
// is not worth a frozen window, so the key is read with a deadline and the
// request goes out without one if it does not come back.
func readSecret(key string) (string, error) {
	if noKeychain() {
		return "", nil
	}
	type result struct {
		v   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := mygo.Secrets.Get(key)
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return string(r.v), r.err
	case <-time.After(3 * time.Second):
		return "", fmt.Errorf("读取凭据超时")
	}
}

// aiClient builds a client from the stored settings.
func (a *App) aiClient() *ai.Client {
	key, _ := readSecret(aiSecretKey)
	return ai.New(ai.Settings{
		BaseURL: a.AIBaseURL(),
		Model:   a.AIModel(),
		APIKey:  key,
		Timeout: 30 * time.Second,
	})
}

// diagClient is the client the failure diagnosis and the interpretations
// send with: the main one renamed to the optional 诊断/解读模型. A blank
// field is the main model itself — no second name on the wire. Mapping
// drafts stay on the main model.
func (a *App) diagClient() *ai.Client {
	return a.aiClient().ForModel(a.AIDiagModel())
}

// AIDraft is one proposed mapping and what the model said about it.
type AIDraft struct {
	Mapping     model.Mapping
	Explanation string
}

// ProbeAI checks the endpoint with one small request.
func (a *App) ProbeAI() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.aiClient().Probe(ctx)
}

// FetchModels lists the models the endpoint offers, for the settings
// picker. The caller passes what the fields hold, so an address typed but
// not yet saved still works; an empty key falls back to the stored one.
func (a *App) FetchModels(baseURL, apiKey string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	key := apiKey
	if key == "" {
		key, _ = readSecret(aiSecretKey)
	}
	c := ai.New(ai.Settings{BaseURL: baseURL, APIKey: key, Timeout: 20 * time.Second})
	models, err := c.Models(ctx)
	if err != nil {
		a.Log.Warn("拉取模型列表失败: %v", err)
		return nil, err
	}
	a.Log.Info("拉取到 %d 个模型", len(models))
	return models, nil
}

// Change asks for a frame, for code outside the frame that changed
// something the view shows.
func (a *App) Change() { a.notify() }
