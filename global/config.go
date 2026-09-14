package global

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var Version = "1.0.0"

// ConfigFile 是应用主配置文件名（位于工作目录）。
const ConfigFile = "config.json"

// GConfig 是应用级配置，修改后会写回 ConfigFile。
//
// 这个结构体在取出之后就不该再被就地修改——改配置一律走 Update，
// 它会复制一份、改副本、原子替换，再落盘。
type GConfig struct {
	Language  string `json:"language"`
	LogDir    string `json:"log_dir"`
	PluginDir string `json:"plugin_dir"`
	DataDir   string `json:"data_dir"`
	// BackupDir 是配置快照的存放目录。
	BackupDir string `json:"backup_dir"`
	// BackupKeep 是每个插件保留的快照份数，超出后自动清理最旧的。
	BackupKeep int `json:"backup_keep"`
	// SourceOverrides 允许用户手动指定某个插件的模型配置文件路径，
	// 键为插件 id，值为绝对路径。留空的插件使用定义里的默认路径。
	SourceOverrides map[string]string `json:"source_overrides,omitempty"`
}

var (
	// config 是当前配置快照。读走原子加载，写走 Update 的复制-替换，
	// 因此任何时刻读到的都是一份自洽、不会被就地改动的配置。
	config atomic.Pointer[GConfig]
	// configWriteMu 串行化 Update，避免两个并发修改互相覆盖。
	configWriteMu sync.Mutex
)

func init() {
	config.Store(defaultConfig())
}

// Config 返回当前配置。返回的指针指向一份不会被就地修改的快照，
// 读取方可以安全地把字段抄下来长期使用。
func Config() *GConfig {
	return config.Load()
}

// Update 修改配置并落盘。
//
// mutate 拿到的是一份副本，随便改；改完统一替换并写回文件。
// 因为读方拿的是快照，界面在改语言的同时读备份目录也不会看到半截状态。
func Update(mutate func(*GConfig)) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()

	current := config.Load()
	next := *current
	// map 是引用类型，值拷贝会与旧快照共享，必须克隆后再改。
	next.SourceOverrides = maps.Clone(current.SourceOverrides)
	if next.SourceOverrides == nil {
		next.SourceOverrides = map[string]string{}
	}

	mutate(&next)
	config.Store(&next)
	return saveConfig(&next)
}

func defaultConfig() *GConfig {
	return &GConfig{
		Language:        "zh-CN",
		LogDir:          "logs",
		PluginDir:       "plugins",
		DataDir:         "data",
		BackupDir:       filepath.Join("data", "backups"),
		BackupKeep:      10,
		SourceOverrides: map[string]string{},
	}
}

// LoadConfig 从工作目录读取配置，缺失或损坏时保持默认值。
func LoadConfig() {
	loaded := *config.Load()

	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		var parsed GConfig
		if json.Unmarshal(data, &parsed) == nil {
			applyLoaded(&loaded, &parsed)
		}
	}
	// 目录类字段统一走一次归一化：config.json 可能是别的平台写的，
	// 在 Windows 上存下来的 "data\\backups" 拿到 Linux 上会被当成一个
	// 带反斜杠的文件名，而不是目录。
	loaded.LogDir = normalizeDir(loaded.LogDir)
	loaded.PluginDir = normalizeDir(loaded.PluginDir)
	loaded.DataDir = normalizeDir(loaded.DataDir)
	loaded.BackupDir = normalizeDir(loaded.BackupDir)
	if loaded.SourceOverrides == nil {
		loaded.SourceOverrides = map[string]string{}
	}

	config.Store(&loaded)
}

// applyLoaded 只把文件里显式给出的字段覆盖到当前值上，缺省项保留默认。
func applyLoaded(dst *GConfig, src *GConfig) {
	if src.Language != "" {
		dst.Language = src.Language
	}
	if src.LogDir != "" {
		dst.LogDir = src.LogDir
	}
	if src.PluginDir != "" {
		dst.PluginDir = src.PluginDir
	}
	if src.DataDir != "" {
		dst.DataDir = src.DataDir
	}
	if src.BackupDir != "" {
		dst.BackupDir = src.BackupDir
	}
	if src.BackupKeep > 0 {
		dst.BackupKeep = src.BackupKeep
	}
	if src.SourceOverrides != nil {
		dst.SourceOverrides = src.SourceOverrides
	}
}

// normalizeDir 把任意平台写法的目录统一成本平台的分隔符。
func normalizeDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.FromSlash(strings.ReplaceAll(p, `\`, "/"))
}

// SaveConfig 把当前配置写回磁盘。
func SaveConfig() error {
	return saveConfig(Config())
}

func saveConfig(cfg *GConfig) error {
	buf, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	tmp := ConfigFile + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ConfigFile)
}

// StatePath 返回状态文件路径。
func StatePath() string {
	return filepath.Join(Config().DataDir, "state.json")
}

// UpstreamPath 返回程序级上游目录文件路径。
func UpstreamPath() string {
	return filepath.Join(Config().DataDir, "upstreams.json")
}
