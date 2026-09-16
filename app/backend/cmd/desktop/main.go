package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	app := NewDesktopApp()

	if err := wails.Run(&options.App{
		Title:  "TradingAgents",
		Width:  1440,
		Height: 960,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 8, G: 9, B: 11, A: 255}, // #08090b，与主体同色
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHidden(),
			Appearance:           mac.NSAppearanceNameDarkAqua, // 原生标题栏跟暗色主体
			WebviewIsTransparent: false,                       // 不透明，避免灰条/色差
			WindowIsTranslucent:  false,
		},
	}); err != nil {
		log.Fatalf("wails failed: %v", err)
	}
}
