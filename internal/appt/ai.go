package appt

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/lianxin255/nginx-jump-gui/internal/ai"
	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/store"
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
func (a *App) SetAI(baseURL, model string) {
	_ = a.Store.SetSetting(store.SettingAIBaseURL, strings.TrimSpace(baseURL))
	_ = a.Store.SetSetting(store.SettingAIModel, strings.TrimSpace(model))
	a.notify()
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

// SetAIKey stores the API key in the system credential store.
//
// The store is written from its own goroutine with a deadline, like the
// read: a locked keychain would otherwise freeze the window.
func (a *App) SetAIKey(key string) error {
	if os.Getenv("NGJ_NO_KEYCHAIN") == "1" {
		return fmt.Errorf("当前环境没有系统凭据库，无法保存 API Key")
	}
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
	if os.Getenv("NGJ_NO_KEYCHAIN") == "1" {
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

// AskAI turns a sentence into a mapping draft.
//
// It returns the model's explanation and the mapped draft. The draft is
// not saved: the caller shows it in the form, and saving goes through the
// ordinary validation.
func (a *App) AskAI(sentence string) (explanation string, draft *model.Mapping, err error) {
	if !a.AIReady() {
		return "", nil, fmt.Errorf("还没有配置 AI 服务，请先在设置里填写")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := a.aiClient()
	d, err := client.Draft(ctx, sentence)
	if err != nil {
		a.Log.Warn("AI 生成失败: %v", err)
		return "", nil, err
	}
	m, explanation, err := d.Mapping()
	if err != nil {
		return explanation, nil, err
	}
	a.Log.Info("AI 生成了草稿「%s」", m.Name)
	return explanation, &m, nil
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
