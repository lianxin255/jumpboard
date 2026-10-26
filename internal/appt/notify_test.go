package appt

import (
	"strings"
	"sync"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
)

// recorder collects what the app would have shown as desktop
// notifications, in place of a desktop.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) notify(title, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, title+"|"+body)
}

func (r *recorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recorder) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return ""
	}
	return r.calls[len(r.calls)-1]
}

// notifiedApp builds an app whose notifications land in a recorder, on a
// throwaway data directory like the e2e harness.
func notifiedApp(t *testing.T) (*App, *recorder) {
	t.Helper()
	t.Setenv("JUMPBOARD_DATA_ROOT", t.TempDir())
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	rec := &recorder{}
	a.Notify = rec.notify
	return a, rec
}

// TestHealthChangeNotifications pins which transitions are worth a
// desktop notification: only a mapping that was answering and stopped.
// The first look at a dead service, and a recovery, stay in the table.
func TestHealthChangeNotifications(t *testing.T) {
	a, rec := notifiedApp(t)
	changed := func(from, to model.HealthStatus, name string) services.HealthChange {
		return services.HealthChange{Mapping: model.Mapping{Name: name}, From: from, To: to}
	}

	a.notifyHealthChanges([]services.HealthChange{
		changed(model.HealthUnknown, model.HealthUnhealthy, "从未通过"),
		changed(model.HealthHealthy, model.HealthUnhealthy, "掉线"),
		changed(model.HealthUnhealthy, model.HealthHealthy, "恢复"),
	})
	if got := rec.len(); got != 1 {
		t.Fatalf("only the healthy→unhealthy move is news, got %d notifications", got)
	}
	if body := rec.last(); !strings.Contains(body, "掉线") || !strings.Contains(body, "健康检查失败") {
		t.Errorf("single outage wording: %q", body)
	}

	// Several rules dropping at once is one banner naming them all.
	a.notifyHealthChanges([]services.HealthChange{
		changed(model.HealthHealthy, model.HealthUnhealthy, "A"),
		changed(model.HealthHealthy, model.HealthUnhealthy, "B"),
	})
	if got := rec.len(); got != 2 {
		t.Fatalf("a batch is one notification, got %d", got)
	}
	if body := rec.last(); !strings.Contains(body, "2 条") || !strings.Contains(body, "A") || !strings.Contains(body, "B") {
		t.Errorf("batch wording: %q", body)
	}
}

// TestApplyNotifies walks the apply notification path both ways: a rule
// set that comes up says so once, and one that cannot start says why.
func TestApplyNotifies(t *testing.T) {
	t.Run("a refused apply", func(t *testing.T) {
		a, rec := notifiedApp(t)
		// A whitelist on with no addresses is refused before the engine
		// runs, which keeps the test off real ports.
		if err := a.Store.SetWhitelistOn(true); err != nil {
			t.Fatal(err)
		}
		if err := a.Apply(); err == nil {
			t.Fatal("the apply should have been refused")
		}
		if rec.len() != 1 {
			t.Fatalf("a failed apply is one notification, got %d", rec.len())
		}
		if body := rec.last(); !strings.Contains(body, "应用规则失败") || !strings.Contains(body, "白名单") {
			t.Errorf("failure wording: %q", body)
		}
	})
	t.Run("a clean apply", func(t *testing.T) {
		a, rec := notifiedApp(t)
		if err := a.Apply(); err != nil {
			t.Fatal(err)
		}
		// No mappings: nothing is listening, which is a shutdown rather
		// than an event, and stays out of the notification centre.
		if rec.len() != 0 {
			t.Errorf("an empty rule set should not notify: %v", rec.calls)
		}
	})
	t.Run("nil Notify is silent", func(t *testing.T) {
		a, _ := notifiedApp(t)
		a.Notify = nil // as in every test that does not care
		if err := a.Store.SetWhitelistOn(true); err != nil {
			t.Fatal(err)
		}
		if err := a.Apply(); err == nil {
			t.Fatal("the apply should have been refused")
		}
	})
}

// TestTruncateNotice keeps a notification's body to a banner's worth.
func TestTruncateNotice(t *testing.T) {
	short := "端口被占用"
	if got := truncateNotice(short, 120); got != short {
		t.Errorf("a short body should pass through: %q", got)
	}
	long := strings.Repeat("错", 300)
	got := truncateNotice(long, 120)
	if n := len([]rune(got)); n != 121 {
		t.Errorf("truncated to %d runes, want 121 (120 + ellipsis)", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("the truncation should end with an ellipsis: %q", got)
	}
}
