package service

import (
	"buddyswitch/api"
	"buddyswitch/global"
)

// GetLangTextMap 返回当前语言的文案映射。
func (a *App) GetLangTextMap() map[string]string {
	return global.GetLangTextMap()
}

// GetLangPack 返回当前语言包。
func (a *App) GetLangPack() *global.LanguagePack {
	pack, err := api.GetLang()
	if err != nil {
		global.Log.Warnf("获取语言包失败: %v", err)
		return nil
	}
	return pack
}

// GetALLLang 返回所有可用语言。
func (a *App) GetALLLang() []global.LanguageInfo {
	return api.GetALLLang()
}

// SetLanguage 切换语言并持久化。
func (a *App) SetLanguage(langCode string) bool {
	if err := global.Update(func(cfg *global.GConfig) {
		cfg.Language = langCode
	}); err != nil {
		global.Log.Warnf("保存语言设置失败: %v", err)
	}
	global.ClearLangCache()
	global.UpdateCurrentLangPath()
	return true
}

// GetCurrentLang 返回当前语言代码。
func (a *App) GetCurrentLang() string {
	return global.Config().Language
}
