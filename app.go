package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"pampp/internal/core"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App exposes the manager to the frontend. Every exported method becomes a JS binding.
type App struct {
	ctx      context.Context
	m        *core.Manager
	quitting bool
	tray     bool
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.m = core.NewManager(core.ResolveRoot(), func(event string, data any) {
		runtime.EventsEmit(ctx, event, data)
	})
	if a.m.AutoStart() {
		go a.m.StartAll()
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if !a.quitting && a.tray && a.m.CloseToTray() {
		runtime.WindowHide(ctx)
		return true
	}
	return false
}

func (a *App) shutdown(ctx context.Context) { a.m.Shutdown() }

func (a *App) show() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) quit() {
	a.quitting = true
	runtime.Quit(a.ctx)
}

// ---- state & services ----

func (a *App) GetState() core.State                { return a.m.State() }
func (a *App) GetCatalog(force bool) *core.Catalog { return a.m.Catalog(force) }
func (a *App) StartAll() error                     { return a.m.StartAll() }
func (a *App) StopAll()                            { a.m.StopAll() }
func (a *App) StartService(id string) error        { return a.m.StartService(id) }
func (a *App) StopService(id string)               { a.m.StopService(id) }
func (a *App) RestartService(id string) error      { return a.m.RestartService(id) }
func (a *App) SaveSettings(cfg core.Config) error  { return a.m.SaveSettings(cfg) }

// ---- sites ----

func (a *App) CreateSite(name, php string) (*core.SiteState, error) {
	return a.m.CreateSite(name, php)
}
func (a *App) SetSitePHP(site, branch string) error { return a.m.SetSitePHP(site, branch) }

// ---- SSL ----

func (a *App) TrustCA() error   { return a.m.TrustCA() }
func (a *App) UntrustCA() error { return a.m.UntrustCA() }

// ---- PHP ----

func (a *App) SetDefaultPHP(branch string) error { return a.m.SetDefaultPHP(branch) }
func (a *App) RemovePHP(branch string) error     { return a.m.RemovePHP(branch) }
func (a *App) RestartPHP(branch string) error    { return a.m.RestartPHP(branch) }
func (a *App) GetExtensions(branch string) ([]core.PHPExtension, error) {
	return a.m.Extensions(branch)
}
func (a *App) SetExtension(branch, name string, on bool) error {
	return a.m.SetExtension(branch, name, on)
}
func (a *App) OpenPHPIni(branch string) error {
	path, err := a.m.PHPIniPath(branch)
	if err != nil {
		return err
	}
	return a.m.Open(path)
}

// ---- installs ----

func (a *App) Install(component, branch string) string { return a.m.Install(component, branch) }
func (a *App) QuickSetup() string                      { return a.m.QuickSetup() }
func (a *App) DismissTask(id string)                   { a.m.DismissTask(id) }

// ---- logs ----

func (a *App) GetLogs() []core.LogFile             { return a.m.Logs() }
func (a *App) ReadLog(name string) (string, error) { return a.m.ReadLog(name, 128*1024) }
func (a *App) ClearLog(name string) error          { return a.m.ClearLog(name) }

// ---- shell ----

func (a *App) OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("geçersiz adres")
	}
	return a.m.Open(url)
}

// OpenFolder opens a folder inside the PAMPP root (www, logs, a site...).
func (a *App) OpenFolder(path string) error {
	root := a.m.P.Root
	if path == "" {
		path = root
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("klasör PAMPP dışında")
	}
	return a.m.Open(path)
}

func (a *App) OpenTerminal(dir string) error { return a.m.OpenTerminal(dir) }
