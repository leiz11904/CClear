//go:build (production || dev) && windows

package main

import (
	"embed"

	"cclear/internal/appinfo"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:ui/dist
var assets embed.FS

func main() {
	application := NewApp()
	_ = wails.Run(&options.App{
		Title:     appinfo.Title,
		Width:     1180,
		Height:    800,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 231, G: 235, B: 240, A: 255},
		OnStartup:        application.startup,
		Bind:             []interface{}{application},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.SystemDefault,
		},
	})
}
