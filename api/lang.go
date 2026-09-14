// Package api 存放不依赖 Wails 上下文的业务函数。
//
// 这一层可以被 service（Wails 绑定）调用，也可以被将来可能出现的命令行
// 入口或自动化脚本直接复用，因此这里不出现 context.Context 与 runtime 调用。
package api

import (
	"buddyswitch/global"
)

// GetLang 返回当前语言包。
func GetLang() (*global.LanguagePack, error) {
	return global.GetLangPack()
}

// GetALLLang 返回全部可用语言。
func GetALLLang() []global.LanguageInfo {
	return global.GetLangInfoList()
}
