package appt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// AskAIFilter asks the model which mappings match one natural language
// condition, and keeps the answer for the table to filter with.
//
// The whole table travels as one summary line per mapping — the fields a
// condition can name, never a secret — and what comes back only narrows
// what the table shows: nothing here saves, applies or writes anything.
// The busy check, the busy flip and the clearing of the old answer are
// one locked step, the way the diagnosis's are: two clicks must not
// start two requests, and a second ask must not leave the first answer
// filtering the table underneath the spinner.
func (a *App) AskAIFilter(query string) error {
	query = strings.TrimSpace(query)
	if !a.AIReady() {
		return fmt.Errorf("还没有配置 AI 服务，请先在设置里填写")
	}
	a.mu.Lock()
	if a.filterBusy {
		a.mu.Unlock()
		return fmt.Errorf("筛选还在进行中")
	}
	a.filterBusy, a.filterErr = true, ""
	a.filterQuery, a.filterRationale, a.filterIDs = "", "", nil
	a.mu.Unlock()

	// The whole table is read from the store, not from what the search
	// box has already hidden: a condition like "上游 192.168" has to see
	// every row the user means, including the ones a search folded away.
	mappings, err := a.Store.ListMappings()
	var res ai.FilterResult
	if err == nil {
		rows := make([]ai.MappingSummary, 0, len(mappings))
		for _, m := range mappings {
			rows = append(rows, ai.MappingSummary{
				ID: m.ID, Name: m.Name, Protocol: m.Protocol.Label(),
				ListenPort: m.ListenPort, ServerNames: m.ServerNames,
				ListenPath: m.ListenPath, UpstreamHost: m.UpstreamHost,
				UpstreamPort: m.UpstreamPort, Enabled: m.Enabled,
			})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		res, err = a.diagClient().FilterMappings(ctx, query, rows)
	}

	a.mu.Lock()
	a.filterBusy = false
	hits := 0
	if err != nil {
		a.filterErr, a.filterQuery, a.filterRationale, a.filterIDs = err.Error(), "", "", nil
	} else {
		// The model's ids are intersected with the mappings that exist:
		// one the ai layer's list did not stop — a mapping deleted while
		// the request was out — must not quietly empty the table.
		live := make(map[string]bool, len(mappings))
		for _, m := range mappings {
			live[m.ID] = true
		}
		ids := make(map[string]bool, len(res.IDs))
		for _, id := range res.IDs {
			if live[id] {
				ids[id] = true
			}
		}
		hits = len(ids)
		a.filterErr = ""
		a.filterQuery, a.filterRationale, a.filterIDs = query, res.Rationale, ids
	}
	a.mu.Unlock()
	if err != nil {
		a.Log.Warn("AI 筛选失败: %v", err)
	} else {
		a.Log.Info("AI 筛选完成，命中 %d 条", hits)
	}
	a.notify()
	return err
}

// AIFilterState returns what the natural-language filter is doing, for
// the mappings page: busy while the request is out, the error when it
// failed, and — once it landed — the query ("" means the filter is off),
// the model's one-line rationale and the set of ids the table keeps.
// The set is copied, so a frame can walk it without holding the lock
// and without the power to change it.
func (a *App) AIFilterState() (busy bool, errText string, query, rationale string, ids map[string]bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.filterIDs != nil {
		ids = make(map[string]bool, len(a.filterIDs))
		for id, on := range a.filterIDs {
			ids[id] = on
		}
	}
	return a.filterBusy, a.filterErr, a.filterQuery, a.filterRationale, ids
}

// ClearAIFilter puts the table back to every row, for the 清除筛选
// button. It retires a landed filter and a landed error alike, so one
// button clears whatever the last ask left behind. An ask that is still
// out is not cancelled — it lands and becomes the filter, which is what
// the click that started it asked for.
func (a *App) ClearAIFilter() {
	a.mu.Lock()
	a.filterBusy, a.filterErr = false, ""
	a.filterQuery, a.filterRationale, a.filterIDs = "", "", nil
	a.mu.Unlock()
	a.notify()
}
