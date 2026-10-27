package appt

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// deeplinkScheme is the URL scheme mygo.json declares, and the only one
// this file answers.
const deeplinkScheme = "jumpboard"

// deeplinkMaxPayload bounds what a deeplink may carry. A URL is a poor
// carrier for a whole backup: past this, the honest answer is to paste it
// instead.
const deeplinkMaxPayload = 4 << 10

// Deeplink is one jumpboard:// URL, read into its parts.
type Deeplink struct {
	// Show reports the URL asked for the window to come forward.
	Show bool
	// Payload is the content an import URL carries, decoded.
	Payload string
	// Err explains why the URL was refused, "" for one the app can act on.
	Err string
}

// ParseDeeplink reads a URL into a Deeplink, without touching the app.
// The two requests it knows are a bare scheme (show the window) and the
// import form with the content in the data query parameter, percent-encoded
// the way any text goes into a URL.
func ParseDeeplink(raw string) Deeplink {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Deeplink{Err: fmt.Sprintf("链接无法解析：%v", err)}
	}
	if !strings.EqualFold(u.Scheme, deeplinkScheme) {
		return Deeplink{Err: fmt.Sprintf("不是 JumpBoard 链接（期望 %s://）", deeplinkScheme)}
	}
	host := strings.ToLower(u.Host)
	switch {
	case host == "" || host == "open":
		return Deeplink{Show: true}
	case host == "import":
		payload := u.Query().Get("data")
		if payload == "" {
			return Deeplink{Err: "导入链接缺少内容"}
		}
		if len(payload) > deeplinkMaxPayload {
			return Deeplink{Err: "链接内容太长（超过 4 KB），请改用粘贴导入"}
		}
		return Deeplink{Payload: payload}
	default:
		return Deeplink{Err: fmt.Sprintf("不认识的 JumpBoard 链接：%s://%s", deeplinkScheme, u.Host)}
	}
}

// HandleDeeplink runs one URL's request: showing the window, or putting
// the content it carries on the import preview. The window is the UI's
// job — ShowWindow points at it — so a deeplink that arrives before the
// window exists simply does nothing for that half.
func (a *App) HandleDeeplink(raw string) error {
	dl := ParseDeeplink(raw)
	if dl.Err != "" {
		return errors.New(dl.Err)
	}
	if dl.Show {
		if a.ShowWindow != nil {
			a.ShowWindow()
		}
		return nil
	}
	return a.StartPasteFromText(dl.Payload)
}
