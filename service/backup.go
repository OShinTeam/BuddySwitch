package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"buddyswitch/backup"
	"buddyswitch/global"
)

// errInvalidBackupName 表示快照文件名不合法（可能试图越权访问目录外的文件）。
func errInvalidBackupName() error {
	return fmt.Errorf("%s", global.T("err_invalid_backup_name"))
}

// errInvalidPluginID 表示插件 id 不合法。
func errInvalidPluginID() error {
	return fmt.Errorf("%s", global.T("err_invalid_plugin_id"))
}

// captureBackup 在写回原生配置之前留一份快照。
func (a *App) captureBackup(pluginID string, file string) {
	cfg := global.Config()
	path, err := backup.Capture(file, cfg.BackupDir, pluginID, cfg.BackupKeep)
	if err != nil {
		global.Log.Warnf("备份 %s 失败: %v", file, err)
		return
	}
	if path != "" {
		global.Log.Debugf("已备份 %s -> %s", file, path)
	}
}

// backupDirFor 返回某个插件的快照目录，并先把 pluginID 卡在合法范围内。
//
// 快照目录是 backup_dir/<插件 id>，插件 id 直接参与拼路径。校验放在这里而不是
// 只放在能 reg.Get 到的地方：插件 id 来自插件定义文件，一个写了 "../" 的
// plugins/*.json 同样能让路径跑出备份根目录。
func backupDirFor(pluginID string) (string, error) {
	if !safePluginID(pluginID) {
		global.Log.Warnf("非法的插件 id: %q", pluginID)
		return "", errInvalidPluginID()
	}
	return backup.Dir(global.Config().BackupDir, pluginID), nil
}

// ListBackups 返回某个插件的全部快照，最新的在前。
func (a *App) ListBackups(pluginID string) ([]backup.Entry, error) {
	if _, err := a.reg.Get(pluginID); err != nil {
		return nil, err
	}
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return nil, err
	}
	return backup.List(dir)
}

// ReadBackup 读取一份快照的内容，用于界面预览。
func (a *App) ReadBackup(pluginID string, name string) (string, error) {
	if !safeName(name) {
		return "", errInvalidBackupName()
	}
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return "", err
	}
	return backup.Read(filepath.Join(dir, name))
}

// RestoreBackup 用指定快照覆盖该插件的模型配置文件。
//
// 覆盖之前会把当前内容再存一份快照，所以还原动作本身也可以撤销。
func (a *App) RestoreBackup(pluginID string, name string) error {
	_, file, err := a.requireSource(pluginID)
	if err != nil {
		return err
	}
	if !safeName(name) {
		return errInvalidBackupName()
	}
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return err
	}
	cfg := global.Config()
	if err := backup.Restore(filepath.Join(dir, name), file, cfg.BackupDir, pluginID, cfg.BackupKeep); err != nil {
		return err
	}
	global.Log.Infof("已用快照 %s 还原 %s", name, file)
	return nil
}

// ClearBackups 清空某个插件的全部快照。
func (a *App) ClearBackups(pluginID string) (int, error) {
	if _, err := a.reg.Get(pluginID); err != nil {
		return 0, err
	}
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return 0, err
	}
	removed, err := backup.Clear(dir)
	if err != nil {
		return 0, err
	}
	global.Log.Infof("已清理 %s 的 %d 份快照", pluginID, removed)
	return removed, nil
}

// OpenBackupDir 在系统文件管理器中打开该插件的快照目录。
func (a *App) OpenBackupDir(pluginID string) bool {
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if err := revealPath(abs); err != nil {
		global.Log.Warnf("打开备份目录失败: %v", err)
		return false
	}
	return true
}

// backupCount 返回某个插件当前的快照数量。
func (a *App) backupCount(pluginID string) int {
	dir, err := backupDirFor(pluginID)
	if err != nil {
		return 0
	}
	items, err := backup.List(dir)
	if err != nil {
		return 0
	}
	return len(items)
}

// safePluginID 拒绝带路径分隔符或上跳片段的插件 id。
func safePluginID(pluginID string) bool {
	return pluginID != "" &&
		pluginID != "." &&
		!strings.ContainsAny(pluginID, `/\`) &&
		!strings.Contains(pluginID, "..")
}

// safeName 拒绝带路径分隔符的快照文件名，防止越权读写。
func safeName(name string) bool {
	return name != "" &&
		!strings.ContainsAny(name, `/\`) &&
		!strings.Contains(name, "..") &&
		strings.HasSuffix(name, ".bak")
}
