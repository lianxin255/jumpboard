package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// Templates lists the built-in and the user's templates.
type Templates struct {
	// customDir holds the user's templates, one YAML file each.
	customDir string
	// builtin are the templates embedded in the app.
	builtin map[string]model.Template
	// mu guards the custom files, which the UI writes from its own
	// goroutine while a health check may be reading them.
	mu sync.RWMutex
}

// NewTemplates returns the templates of the app: builtin are the embedded
// YAML files, customDir is where the user's own live.
func NewTemplates(builtin map[string]model.Template, customDir string) *Templates {
	return &Templates{builtin: builtin, customDir: customDir}
}

// List returns every template, the built-in ones first, sorted by the way
// the picker groups them.
func (t *Templates) List() []model.Template {
	out := make([]model.Template, 0, len(t.builtin))
	for _, tpl := range t.builtin {
		tpl.Builtin = true
		out = append(out, tpl)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})

	t.mu.RLock()
	defer t.mu.RUnlock()
	entries, err := os.ReadDir(t.customDir)
	if err != nil {
		return out
	}
	var custom []model.Template
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		tpl, err := readTemplate(filepath.Join(t.customDir, name))
		if err != nil {
			continue // a broken file is skipped, not fatal
		}
		if tpl.ID == "" {
			tpl.ID = strings.TrimSuffix(strings.TrimSuffix(name, ".yml"), ".yaml")
		}
		tpl.Builtin = false
		custom = append(custom, tpl)
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(out, custom...)
}

// Builtin returns only the templates that ship with the app.
func (t *Templates) Builtin() []model.Template {
	out := make([]model.Template, 0, len(t.builtin))
	for _, tpl := range t.builtin {
		tpl.Builtin = true
		out = append(out, tpl)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// validID allows letters, digits, underscores and dashes: the ID becomes a
// file name, so anything that could walk out of the directory is refused.
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// Save stores a user template.
func (t *Templates) Save(tpl model.Template) error {
	if !validID(tpl.ID) {
		return errors.New("模板 id 只能包含字母、数字、下划线和短横线（1-64 个字符）")
	}
	if tpl.Mapping.Protocol == "" || !tpl.Mapping.Protocol.Valid() {
		return fmt.Errorf("模板 %q 的协议无效", tpl.ID)
	}
	data, err := yaml.Marshal(tpl)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := os.MkdirAll(t.customDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(t.customDir, tpl.ID+".yml"), data, 0o644)
}

// Delete removes a user template.
func (t *Templates) Delete(id string) error {
	if !validID(id) {
		return errors.New("模板 id 非法")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	err := os.Remove(filepath.Join(t.customDir, id+".yml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// readTemplate loads one YAML file.
func readTemplate(path string) (model.Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Template{}, err
	}
	var tpl model.Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return model.Template{}, err
	}
	if tpl.Mapping.Protocol == "" {
		return model.Template{}, fmt.Errorf("%s: no protocol", path)
	}
	return tpl, nil
}

// LoadBuiltin parses the embedded template files.
func LoadBuiltin(files map[string][]byte) (map[string]model.Template, error) {
	out := make(map[string]model.Template, len(files))
	for name, data := range files {
		var tpl model.Template
		if err := yaml.Unmarshal(data, &tpl); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if tpl.ID == "" {
			base := filepath.Base(name)
			tpl.ID = strings.TrimSuffix(strings.TrimSuffix(base, ".yml"), ".yaml")
		}
		tpl.Builtin = true
		out[tpl.ID] = tpl
	}
	return out, nil
}

// ParseTemplate reads a template from bytes, as the import flow does.
func ParseTemplate(data []byte) (model.Template, error) {
	var tpl model.Template
	if err := yaml.Unmarshal(data, &tpl); err != nil {
		return model.Template{}, err
	}
	if tpl.Mapping.Protocol == "" || !tpl.Mapping.Protocol.Valid() {
		return model.Template{}, errors.New("模板缺少有效的协议")
	}
	return tpl, nil
}
