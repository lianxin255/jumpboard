package appt

import (
	"context"
	"fmt"
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
	b, err := mygo.Secrets.Get(aiSecretKey)
	return err == nil && len(b) > 0
}

// SetAIKey stores the API key in the system credential store.
func (a *App) SetAIKey(key string) error {
	if strings.TrimSpace(key) == "" {
		_ = mygo.Secrets.Delete(aiSecretKey)
		a.notify()
		return nil
	}
	if err := mygo.Secrets.Set(aiSecretKey, []byte(key)); err != nil {
		return fmt.Errorf("保存 API Key 失败：%w", err)
	}
	a.notify()
	return nil
}

// aiClient builds a client from the stored settings.
func (a *App) aiClient() *ai.Client {
	key := ""
	if b, err := mygo.Secrets.Get(aiSecretKey); err == nil {
		key = string(b)
	}
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

// Change asks for a frame, for code outside the frame that changed
// something the view shows.
func (a *App) Change() { a.notify() }
