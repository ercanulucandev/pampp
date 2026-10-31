package main

import (
	"context"
	"embed"
	"os"

	"pampp/internal/core"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	// Elevated helper mode: rewrite the hosts file and exit.
	if len(os.Args) == 3 && os.Args[1] == "--sync-hosts" {
		if err := core.SyncHostsFromFile(os.Args[2]); err != nil {
			os.Exit(1)
		}
		return
	}

	core.InitJobObject()
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "PAMPP",
		Width:     1120,
		Height:    740,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 21, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			startTray(app, trayIcon)
		},
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "pampp-6f1d2c84-local-stack",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.show() },
		},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
