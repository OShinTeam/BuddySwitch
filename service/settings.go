package service

import (
	"buddyswitch/backup"
	"buddyswitch/global"
)

// Settings 是面向界面的设置视图。
type Settings struct {
	Language        string            `json:"language"`
	LogLevel        string            `json:"log_level"`
	PluginDir       string            `json:"plugin_dir"`
	DataDir         string            `json:"data_dir"`
	BackupDir       string            `json:"backup_dir"`
	BackupKeep      int               `json:"backup_keep"`
	WorkDir         string            `json:"work_dir"`
	SourceOverrides map[string]string `json:"source_overrides"`
}

// GetSettings 返回当前设置。
func (a *App) GetSettings() Settings {
	cfg := global.Config()
	overrides := make(map[string]string, len(cfg.SourceOverrides))
	for k, v := range cfg.SourceOverrides {
		overrides[k] = v
	}
	return Settings{
		Language:        cfg.Language,
		LogLevel:        a.GetLogLevel(),
		PluginDir:       a.GetPluginDir(),
		DataDir:         cfg.DataDir,
		BackupDir:       cfg.BackupDir,
		BackupKeep:      cfg.BackupKeep,
		WorkDir:         a.GetSystemInfo().WorkDir,
		SourceOverrides: overrides,
	}
}

// UpdateSettings 保存设置中的可写项（语言、日志等级、快照保留份数）。
func (a *App) UpdateSettings(settings Settings) bool {
	changed := false
	if settings.Language != "" && settings.Language != a.GetCurrentLang() {
		a.SetLanguage(settings.Language)
		changed = true
	}
	if settings.LogLevel != "" && settings.LogLevel != a.GetLogLevel() {
		a.SetLogLevel(settings.LogLevel)
		changed = true
	}
	// 保留份数收敛到合法区间后再比较，避免把用户的 0 当成「改成 10 份」。
	if keep := backup.ClampKeep(settings.BackupKeep); keep != global.Config().BackupKeep {
		if err := global.Update(func(cfg *global.GConfig) { cfg.BackupKeep = keep }); err != nil {
			global.Log.Warnf("保存配置失败: %v", err)
		}
		changed = true
	}
	if changed {
		global.Log.Info("设置已保存")
	}
	return changed
}

// RevealPath 在系统文件管理器中定位一个文件或目录。
func (a *App) RevealPath(path string) bool {
	if path == "" {
		return false
	}
	if err := revealPath(path); err != nil {
		global.Log.Warnf("打开路径失败: %v", err)
		return false
	}
	return true
}
