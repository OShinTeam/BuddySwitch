package main

import (
	"embed"

	"buddyswitch/global"
	"buddyswitch/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed lang/*
var langFS embed.FS

func main() {
	global.LangFS = langFS
	global.Init()

	app := service.NewApp()

	err := wails.Run(&options.App{
		Title:     "BuddySwitch",
		Width:     1180,
		Height:    780,
		MinWidth:  940,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// 前端是浅色主题，窗口底色跟随，避免加载瞬间的闪烁。
		BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 1},
		Frameless:        true,
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
