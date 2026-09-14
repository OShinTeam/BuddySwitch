package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"buddyswitch/global"
	"buddyswitch/plugin"
)

// PluginView 是插件定义面向界面的视图，合并了定义内容与运行期状态。
type PluginView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Vendor      string   `json:"vendor"`
	Color       string   `json:"color"`
	Version     string   `json:"version"`
	Builtin     bool     `json:"builtin"`
	Enabled     bool     `json:"enabled"`
	Loaded      bool     `json:"loaded"`
	LoadError   string   `json:"load_error,omitempty"`
	DefPath     string   `json:"def_path"`
	SourceFile  string   `json:"source_file"`
	SourceReady bool     `json:"source_ready"`
	Candidates  []string `json:"candidates"`
	ModelCount  int      `json:"model_count"`
	BackupCount int      `json:"backup_count"`
	ProbeType   string   `json:"probe_type"`
	CanToggle   bool     `json:"can_toggle_enabled"`
	// MappedFields 是该插件在 schema.fields 里真正映射到的统一字段键
	// （路径为空的不算）。前端据此决定编辑器显示哪些输入框——
	// 展示一个写不进去的输入框，比不展示它更糟。
	MappedFields []string `json:"mapped_fields"`
	// Capabilities 是该插件在 schema 里声明过的能力键（如 tool_call），
	// 前端据此渲染能力开关，即使某个模型还没有这些字段也能编辑。
	Capabilities []string `json:"capabilities"`
}

// ListPlugins 返回全部插件及其当前状态。
func (a *App) ListPlugins() []PluginView {
	defs := a.reg.List()
	out := make([]PluginView, 0, len(defs))
	for _, def := range defs {
		out = append(out, a.toView(def))
	}
	return out
}

// ReloadPlugins 重新扫描插件目录并返回最新列表。
func (a *App) ReloadPlugins() []PluginView {
	if err := a.reg.Load(); err != nil {
		global.Log.Errorf("重新加载插件失败: %v", err)
	}
	global.Log.Info("插件已重新加载")
	return a.ListPlugins()
}

func (a *App) toView(def *plugin.Definition) PluginView {
	file, ready := a.sourceFileFor(def)
	count := 0
	if ready {
		if doc, err := plugin.LoadDocument(def, file); err == nil {
			if models, err := doc.Models(); err == nil {
				count = len(models)
			}
		}
	}
	return PluginView{
		ID:           def.ID,
		Name:         def.Name,
		Description:  def.Description,
		Vendor:       def.Vendor,
		Color:        def.Color,
		Version:      def.Version,
		Builtin:      def.Builtin,
		Enabled:      a.state.PluginEnabled(def.ID, def.DefaultEnabled()),
		Loaded:       def.Loaded,
		LoadError:    def.LoadError,
		DefPath:      def.DefPath,
		SourceFile:   file,
		SourceReady:  ready,
		Candidates:   def.SourceFiles,
		ModelCount:   count,
		BackupCount:  a.backupCount(def.ID),
		ProbeType:    def.Probe.Type,
		CanToggle:    def.CanPersistEnabled(),
		MappedFields: def.MappedFieldKeys(),
		Capabilities: def.CapabilityKeys(),
	}
}

// sourceFileFor 解析插件实际使用的模型配置文件路径。
// 优先使用用户在设置中指定的路径，其次取第一个真实存在的候选路径；
// 都不存在时返回首选候选路径与 false，便于「首次创建」时直接写入。
func (a *App) sourceFileFor(def *plugin.Definition) (string, bool) {
	if override := global.Config().SourceOverrides[def.ID]; override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, true
		}
		return override, false
	}
	for _, candidate := range def.SourceFiles {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	if len(def.SourceFiles) > 0 {
		return def.SourceFiles[0], false
	}
	return "", false
}

// requireSource 返回插件与其配置文件路径，插件未启用或配置缺失时报错。
func (a *App) requireSource(pluginID string) (*plugin.Definition, string, error) {
	def, err := a.reg.Get(pluginID)
	if err != nil {
		return nil, "", err
	}
	if !def.Loaded {
		return nil, "", fmt.Errorf("%s", global.T("err_plugin_invalid", def.ID, def.LoadError))
	}
	file, _ := a.sourceFileFor(def)
	if file == "" {
		return nil, "", fmt.Errorf("%s", global.T("err_plugin_no_source", def.ID))
	}
	return def, file, nil
}

// SetPluginEnabled 启用或停用某个插件。停用后其模型不再参与列表与批量测试。
func (a *App) SetPluginEnabled(pluginID string, enabled bool) bool {
	if _, err := a.reg.Get(pluginID); err != nil {
		global.Log.Warnf("%v", err)
		return false
	}
	if err := a.state.SetPluginEnabled(pluginID, enabled); err != nil {
		global.Log.Warnf("保存插件状态失败: %v", err)
		return false
	}
	global.Log.Infof("插件 %s 已%s", pluginID, map[bool]string{true: "启用", false: "停用"}[enabled])
	return true
}

// SetSourcePath 为插件指定自定义的模型配置文件路径，传空字符串恢复默认。
func (a *App) SetSourcePath(pluginID string, path string) bool {
	if _, err := a.reg.Get(pluginID); err != nil {
		global.Log.Warnf("%v", err)
		return false
	}
	if err := global.Update(func(cfg *global.GConfig) {
		if path == "" {
			delete(cfg.SourceOverrides, pluginID)
			return
		}
		cfg.SourceOverrides[pluginID] = path
	}); err != nil {
		global.Log.Warnf("保存配置失败: %v", err)
		return false
	}
	global.Log.Infof("插件 %s 配置文件已设为 %s", pluginID, path)
	return true
}

// GetPluginDir 返回插件定义目录的绝对路径。
func (a *App) GetPluginDir() string {
	abs, err := filepath.Abs(a.reg.Dir())
	if err != nil {
		return a.reg.Dir()
	}
	return abs
}

// OpenPluginDir 在系统文件管理器中打开插件目录。
func (a *App) OpenPluginDir() bool {
	dir := a.GetPluginDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		global.Log.Warnf("创建插件目录失败: %v", err)
		return false
	}
	if err := revealPath(dir); err != nil {
		global.Log.Warnf("打开插件目录失败: %v", err)
		return false
	}
	return true
}

// ExportExamplePlugin 在插件目录写出示例定义，供用户照着扩展新 Agent。
func (a *App) ExportExamplePlugin() (string, error) {
	dir := a.GetPluginDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("%s: %w", global.T("err_create_plugin_dir"), err)
	}
	path := filepath.Join(dir, "example-agent.json")
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s", global.T("err_example_exists", path))
	}
	def := plugin.ExampleDefinition()
	buf, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
		return "", err
	}
	global.Log.Infof("已导出示例插件定义: %s", path)
	return path, nil
}
