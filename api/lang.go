package api

import (
	"wails-temp/global"
)

func GetLang() (*global.LanguagePack, error) {
	return global.GetLangPack()
}

func GetALLLang() []global.LanguageInfo {
	return global.GetLangInfoList()
}