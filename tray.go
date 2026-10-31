package main

import (
	"runtime"

	"github.com/energye/systray"
)

// startTray runs the notification-area icon on its own OS thread.
func startTray(app *App, icon []byte) {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTitle("PAMPP")
			systray.SetTooltip("PAMPP")
			systray.SetOnClick(func(systray.IMenu) { app.show() })
			systray.SetOnRClick(func(menu systray.IMenu) { menu.ShowMenu() })

			systray.AddMenuItem("Paneli aç", "").Click(app.show)
			systray.AddSeparator()
			systray.AddMenuItem("Tümünü başlat", "").Click(func() { go app.m.StartAll() })
			systray.AddMenuItem("Tümünü durdur", "").Click(func() { go app.m.StopAll() })
			systray.AddMenuItem("www klasörü", "").Click(func() { app.OpenFolder("www") })
			systray.AddMenuItem("Terminal", "").Click(func() { app.OpenTerminal("") })
			systray.AddSeparator()
			systray.AddMenuItem("Çıkış", "").Click(app.quit)
			app.tray = true
		}, func() {})
	}()
}
