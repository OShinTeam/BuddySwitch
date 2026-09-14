package api

import (
	"buddyswitch/global"
)

// LogInfo 记录一般信息。
func LogInfo(msg string) { global.Log.Info(msg) }

// LogWarn 记录警告信息。
func LogWarn(msg string) { global.Log.Warn(msg) }

// LogError 记录错误信息。
func LogError(msg string) { global.Log.Error(msg) }

// LogDebug 记录调试信息。
func LogDebug(msg string) { global.Log.Debug(msg) }
