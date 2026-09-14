package global

import "os"

// Init 准备运行环境：读配置、建目录、起日志、装语言。
// 必须在任何业务代码（尤其是插件加载）之前调用一次。
func Init() {
	LoadConfig()
	cfg := Config()

	dirs := []string{
		"lang",
		cfg.LogDir,
		cfg.PluginDir,
		cfg.DataDir,
		cfg.BackupDir,
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, 0755)
		}
	}

	InitLogger()
	Log.Info("日志系统初始化完成")

	InitLang()
	Log.Infof("语言系统初始化完成，当前语言: %s", CurrentLangDir())

	Log.Infof("插件目录: %s", cfg.PluginDir)
	Log.Infof("快照目录: %s（每插件保留 %d 份）", cfg.BackupDir, cfg.BackupKeep)
}
