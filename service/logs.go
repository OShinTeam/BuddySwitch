package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"buddyswitch/global"

	"github.com/sirupsen/logrus"
)

// GetLogFiles 返回日志文件列表（新的在前）。
func (a *App) GetLogFiles() []string {
	entries, err := os.ReadDir(global.Config().LogDir)
	if err != nil {
		global.Log.Warnf("读取日志目录失败: %v", err)
		return []string{}
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			files = append(files, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// GetLogFileContent 读取指定日志文件内容。
func (a *App) GetLogFileContent(filename string) string {
	// 安全检查：防止路径遍历
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		global.Log.Warnf("非法的日志文件名: %s", filename)
		return ""
	}
	data, err := os.ReadFile(filepath.Join(global.Config().LogDir, filename))
	if err != nil {
		global.Log.Warnf("读取日志文件失败: %v", err)
		return ""
	}
	return string(data)
}

// SetLogLevel 动态调整日志等级。
func (a *App) SetLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug":
		global.SetLogLevel(logrus.DebugLevel)
	case "info":
		global.SetLogLevel(logrus.InfoLevel)
	case "warn":
		global.SetLogLevel(logrus.WarnLevel)
	case "error":
		global.SetLogLevel(logrus.ErrorLevel)
	default:
		global.Log.Warnf("未知的日志等级: %s", level)
		return false
	}
	global.Log.Infof("日志等级已切换为 %s", strings.ToUpper(level))
	return true
}

// GetLogLevel 返回当前日志等级。
func (a *App) GetLogLevel() string {
	if global.Log == nil {
		return "INFO"
	}
	return strings.ToUpper(global.Log.GetLevel().String())
}
