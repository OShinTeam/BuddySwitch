package plugin

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"buddyswitch/global"
)

// builtinFS 内嵌随程序分发的默认插件定义。
//
//go:embed builtin/*.json
var builtinFS embed.FS

// Registry 负责收集内置插件与磁盘插件，并向业务层提供查询入口。
//
// 加载顺序：先内置，后磁盘。磁盘上同 id 的插件定义会覆盖内置定义
// （方便用户在不重新编译的前提下修正字段映射），不同 id 的则作为新增插件。
type Registry struct {
	dir string
	mu  sync.RWMutex
	def map[string]*Definition
}

// NewRegistry 创建注册表。dir 为磁盘插件目录，为空时使用 ./plugins。
func NewRegistry(dir string) *Registry {
	if strings.TrimSpace(dir) == "" {
		dir = "plugins"
	}
	return &Registry{dir: dir, def: map[string]*Definition{}}
}

// Dir 返回磁盘插件目录。
func (r *Registry) Dir() string { return r.dir }

// Load 重新加载全部插件定义。
func (r *Registry) Load() error {
	loaded := map[string]*Definition{}
	if err := r.loadBuiltin(loaded); err != nil {
		return err
	}
	r.loadDisk(loaded)

	// 补全运行时信息：当前平台的候选配置文件路径与定义校验结果。
	for _, def := range loaded {
		def.SourceFiles = def.ResolvePaths(runtime.GOOS)
		if err := def.Validate(); err != nil {
			def.LoadError = err.Error()
			def.Loaded = false
			continue
		}
		def.Loaded = true
	}

	r.mu.Lock()
	r.def = loaded
	r.mu.Unlock()
	return nil
}

func (r *Registry) loadBuiltin(into map[string]*Definition) error {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return fmt.Errorf("%s: %w", global.T("err_read_builtin_failed"), err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		name := "builtin/" + e.Name()
		data, err := builtinFS.ReadFile(name)
		if err != nil {
			continue
		}
		def, err := parseDefinition(data)
		if err != nil {
			continue
		}
		def.Builtin = true
		def.DefPath = "embedded:" + name
		into[def.ID] = def
	}
	return nil
}

func (r *Registry) loadDisk(into map[string]*Definition) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return // 目录不存在属正常情况
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(r.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		def, err := parseDefinition(data)
		if err != nil || def.ID == "" {
			continue
		}
		def.Builtin = false
		def.DefPath = path
		into[def.ID] = def
	}
}

func parseDefinition(data []byte) (*Definition, error) {
	var def Definition
	if err := json.Unmarshal(stripJSONC(data), &def); err != nil {
		return nil, err
	}
	def.ID = strings.TrimSpace(def.ID)
	return &def, nil
}

// List 返回按 order 与名称排序的插件定义。
func (r *Registry) List() []*Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Definition, 0, len(r.def))
	for _, d := range r.def {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Get 按 id 获取插件定义。
func (r *Registry) Get(id string) (*Definition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.def[id]
	if !ok {
		return nil, fmt.Errorf("%s", global.T("err_plugin_not_found", id))
	}
	return d, nil
}

// ExampleDefinition 返回一份带注释的示例定义，用于在插件目录中引导用户扩展。
func ExampleDefinition() *Definition {
	enabled := true
	return &Definition{
		ID:          "my-agent",
		Name:        "My Agent",
		Description: "示例：把任意 agent 的模型配置接入 BuddySwitch",
		Vendor:      "Your Company",
		Color:       "#8B5CF6",
		Version:     "1.0.0",
		Order:       100,
		Enabled:     &enabled,
		Source: SourceSpec{
			Format: "json",
			Paths: map[string][]string{
				"windows": {`%APPDATA%\MyAgent\models.json`, `%USERPROFILE%\.my-agent\models.json`},
				"darwin":  {"~/Library/Application Support/MyAgent/models.json", "~/.my-agent/models.json"},
				"linux":   {"~/.config/my-agent/models.json", "~/.my-agent/models.json"},
			},
		},
		Schema: SchemaSpec{
			Container: ContainerList{"", "models"},
			Fields: map[string]string{
				FieldID:          "id",
				FieldDisplayName: "name",
				FieldProvider:    "vendor",
				FieldBaseURL:     "url",
				FieldAPIKey:      "apiKey",
				FieldEnabled:     "enabled",
				FieldDescription: "description",
				FieldTags:        "tags",
			},
			Capabilities: map[string]string{
				"tool_call": "supportsToolCall",
				"images":    "supportsImages",
				"reasoning": "supportsReasoning",
			},
			Defaults: map[string]any{"vendor": "Custom"},
		},
		Probe: ProbeSpec{
			Type:       "openai",
			Endpoint:   "{{base_url}}",
			EnsurePath: "/chat/completions",
			Method:     "POST",
			Headers:    map[string]string{"Authorization": "Bearer {{api_key}}"},
			Body: map[string]any{
				"model":      "{{id}}",
				"messages":   []any{map[string]any{"role": "user", "content": "ping"}},
				"max_tokens": 1,
				"stream":     false,
			},
			TimeoutMs: 15000,
		},
		Listing: ListingSpec{
			Endpoint:    "{{base_url}}",
			StripSuffix: "/chat/completions",
			EnsurePath:  "/models",
			Method:      "GET",
			Headers:     map[string]string{"Authorization": "Bearer {{api_key}}"},
			TimeoutMs:   20000,
			ArrayPath:   "data",
			IDField:     "id",
			NameField:   "id",
			NoteField:   "owned_by",
		},
	}
}
