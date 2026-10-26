package native

import (
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// mapping builds one runnable mapping for the summary tests; tests that
// care about a missing port pass 0 for it.
func mapping(name string, enabled bool, health model.HealthStatus) model.Mapping {
	return model.Mapping{Name: name, Enabled: enabled, ListenPort: 9000, HealthStatus: health}
}

// unported is a mapping without a listen port, which the summary must
// not count as a running rule.
func unported(name string) model.Mapping {
	m := mapping(name, true, model.HealthHealthy)
	m.ListenPort = 0
	return m
}

// TestFlyoutSummary pins the panel's headline: the running state with
// the rule count, and the worst finding — an outage before unsaved
// changes, unsaved changes before calm.
func TestFlyoutSummary(t *testing.T) {
	cases := []struct {
		name             string
		engine           string
		mappings         []model.Mapping
		dirty            bool
		wantHeadlineIn   []string
		wantDetail       string
		wantDetailDanger bool
	}{
		{
			name:           "running and clean",
			engine:         "running",
			mappings:       []model.Mapping{mapping("MySQL", true, model.HealthHealthy)},
			wantHeadlineIn: []string{"转发运行中", "1 条映射"},
			wantDetail:     "规则与配置一致",
		},
		{
			name:           "stopped engine says so",
			engine:         "stopped",
			mappings:       []model.Mapping{},
			wantHeadlineIn: []string{"转发已停止", "0 条映射"},
			wantDetail:     "规则与配置一致",
		},
		{
			name:             "an outage outranks dirty",
			engine:           "running",
			mappings:         []model.Mapping{mapping("A", true, model.HealthUnhealthy), mapping("B", true, model.HealthHealthy)},
			dirty:            true,
			wantHeadlineIn:   []string{"2 条映射"},
			wantDetail:       "1 条映射健康检查失败",
			wantDetailDanger: true,
		},
		{
			name:           "dirty when nothing is down",
			engine:         "running",
			mappings:       []model.Mapping{mapping("A", true, model.HealthHealthy)},
			dirty:          true,
			wantHeadlineIn: []string{"1 条映射"},
			wantDetail:     "有未应用的更改",
		},
		{
			name:           "disabled mappings do not count as runnable",
			engine:         "running",
			mappings:       []model.Mapping{mapping("停用", false, model.HealthHealthy), unported("无端口")},
			wantHeadlineIn: []string{"0 条映射"},
			wantDetail:     "规则与配置一致",
		},
		{
			name:           "unknown health is not an outage",
			engine:         "running",
			mappings:       []model.Mapping{mapping("未查", true, model.HealthUnknown)},
			wantHeadlineIn: []string{"1 条映射"},
			wantDetail:     "规则与配置一致",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headline, detail := flyoutSummary(tc.engine, tc.mappings, tc.dirty)
			for _, want := range tc.wantHeadlineIn {
				if !strings.Contains(headline, want) {
					t.Errorf("headline %q should contain %q", headline, want)
				}
			}
			if detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", detail, tc.wantDetail)
			}
			if tc.wantDetailDanger && detail == "规则与配置一致" {
				t.Errorf("an outage must not read as calm")
			}
		})
	}
}
