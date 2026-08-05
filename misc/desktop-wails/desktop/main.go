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
		BackgroundColour: &options.RGBA{R: 8, G: 9, B: 11, A: 1}, // Matches app #08090b background
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true, // Make titlebar background transparent
				HideTitle:                  false, // Keep window title visible
				FullSizeContent:            true,  // Extend webview content under titlebar
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	}); err != nil {
		log.Fatalf("wails failed: %v", err)
	}
}
