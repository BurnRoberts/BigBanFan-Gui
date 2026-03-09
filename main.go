package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

// icon is embedded at compile time from build/appicon.png.
// On Linux, Wails reads the window icon exclusively from options.Linux.Icon;
// the PNG file alone is only used during Windows/macOS packaging.
//
//go:embed build/appicon.png
var icon []byte

// version is injected at build time via:
//
//	-ldflags "-X 'main.version=v1.0.0'"
//
// Update this value when cutting a new release.
var version = "v1.0.0"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "BigBanFan " + version,
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 1}, // #0d1117 GitHub dark
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon: icon,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
