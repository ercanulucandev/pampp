package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------- Node.js runtime catalog ----------------

type NodeRelease struct {
	Branch  string `json:"branch"` // major, e.g. "24"
	Version string `json:"version"`
	LTS     string `json:"lts"` // codename, empty for "Current"
	URL     string `json:"url"`
}

// fetchNodeReleases returns the newest release of each recent major line.
func fetchNodeReleases() ([]NodeRelease, error) {
	text, err := fetchText("https://nodejs.org/dist/index.json")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Version string          `json:"version"`
		Files   []string        `json:"files"`
		LTS     json.RawMessage `json:"lts"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []NodeRelease
	for _, r := range raw { // index.json is newest first
		v := strings.TrimPrefix(r.Version, "v")
		if strings.Contains(v, "-") || !contains(r.Files, "win-x64-zip") {
			continue // pre-releases or no Windows build
		}
		major := strings.SplitN(v, ".", 2)[0]
		if seen[major] {
			continue
		}
		seen[major] = true
		var lts string
		_ = json.Unmarshal(r.LTS, &lts) // false for Current lines
		out = append(out, NodeRelease{Branch: major, Version: v, LTS: lts,
			URL: fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-win-x64.zip", v, v)})
		if len(out) == 5 {
			break
		}
	}
	return out, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// installedNode returns installed Node majors (newest first) and their versions.
func (m *Manager) installedNode() ([]string, map[string]string) {
	versions := map[string]string{}
	var majors []string
	entries, _ := os.ReadDir(m.P.Bin("node"))
	for _, e := range entries {
		dir := m.P.Bin("node", e.Name())
		if e.IsDir() && !strings.Contains(e.Name(), ".") && exists(filepath.Join(dir, "node.exe")) {
			majors = append(majors, e.Name())
			versions[e.Name()] = readVersionFile(dir)
		}
	}
	sort.Slice(majors, func(i, j int) bool { return compareVersions(majors[i], majors[j]) > 0 })
	return majors, versions
}

// defaultNode prefers the configured major, then the newest LTS-looking (even) major.
func (m *Manager) defaultNode(cfg Config, installed []string) string {
	for _, b := range installed {
		if b == cfg.DefaultNode {
			return b
		}
	}
	for _, b := range installed {
		var n int
		fmt.Sscan(b, &n)
		if n%2 == 0 {
			return b
		}
	}
	if len(installed) > 0 {
		return installed[0]
	}
	return ""
}

func (m *Manager) installNode(t *Task, branch string) error {
	var rel *NodeRelease
	for _, r := range m.Catalog(false).Node {
		if r.Branch == branch {
			r := r
			rel = &r
		}
	}
	if rel == nil {
		return fmt.Errorf("Node %s bulunamadı", branch)
	}
	zipPath := m.P.Tmp("downloads", filepath.Base(rel.URL))
	if err := m.fetch(t, "Node.js "+rel.Version, rel.URL, "", zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)
	m.updateTask(t, func(t *Task) { t.Stage = "Node.js " + rel.Version + " kuruluyor"; t.Progress = -1 })
	dir := m.P.Bin("node", rel.Branch)
	if err := installArchive(zipPath, dir, "node.exe", nil); err != nil {
		return err
	}
	if cfg := m.config(); cfg.DefaultNode == "" {
		m.updateConfig(func(c *Config) { c.DefaultNode = rel.Branch })
	}
	return writeVersionFile(dir, rel.Version)
}

func (m *Manager) SetDefaultNode(branch string) error {
	err := m.updateConfig(func(c *Config) { c.DefaultNode = branch })
	m.changed()
	return err
}

func (m *Manager) RemoveNode(branch string) error {
	dir := m.P.Bin("node", branch)
	if !siteNameRe.MatchString(branch) || !exists(filepath.Join(dir, "node.exe")) {
		return fmt.Errorf("Node %s kurulu değil", branch)
	}
	for key, r := range m.nodeRunsSnapshot() {
		if r.running() && r.node == branch {
			return fmt.Errorf("Node %s şu an %s tarafından kullanılıyor", branch, key)
		}
	}
	err := os.RemoveAll(dir)
	m.changed()
	return err
}

// ---------------- projects ----------------

type NodeScript struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Running  bool   `json:"running"`
	URL      string `json:"url"`
	ExitCode *int   `json:"exitCode"`
}

type NodeApp struct {
	Name        string       `json:"name"`
	Path        string       `json:"path"`
	PackageName string       `json:"packageName"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Framework   string       `json:"framework"`
	Manager     string       `json:"manager"` // npm | pnpm | yarn | bun
	HasModules  bool         `json:"hasModules"`
	Node        string       `json:"node"` // major used to run it
	Installing  bool         `json:"installing"`
	Scripts     []NodeScript `json:"scripts"`
}

type NodeState struct {
	Installed []NodeVersionState `json:"installed"`
	Default   string             `json:"default"`
	Apps      []NodeApp          `json:"apps"`
	Running   int                `json:"running"`
}

type NodeVersionState struct {
	Branch  string `json:"branch"`
	Version string `json:"version"`
	Default bool   `json:"default"`
}

// frameworks are checked in order; the first dependency found wins.
var frameworks = []struct{ dep, name string }{
	{"next", "Next.js"}, {"nuxt", "Nuxt"}, {"@remix-run/react", "Remix"}, {"@sveltejs/kit", "SvelteKit"},
	{"astro", "Astro"}, {"@angular/core", "Angular"}, {"@nestjs/core", "NestJS"}, {"gatsby", "Gatsby"},
	{"vite", "Vite"}, {"react-scripts", "Create React App"}, {"electron", "Electron"},
	{"express", "Express"}, {"fastify", "Fastify"}, {"koa", "Koa"}, {"hono", "Hono"},
}

// scriptOrder puts the scripts people click most at the front.
var scriptOrder = map[string]int{"dev": 0, "start": 1, "serve": 2, "build": 3, "preview": 4, "test": 5, "lint": 6}

func (m *Manager) nodeApps(cfg Config) []NodeApp {
	installed, _ := m.installedNode()
	def := m.defaultNode(cfg, installed)
	has := map[string]bool{}
	for _, b := range installed {
		has[b] = true
	}
	runs := m.nodeRunsSnapshot()

	apps := []NodeApp{}
	entries, _ := os.ReadDir(m.P.Node())
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := m.P.Node(e.Name())
		b, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			continue
		}
		var pkg struct {
			Name            string            `json:"name"`
			Version         string            `json:"version"`
			Description     string            `json:"description"`
			Scripts         map[string]string `json:"scripts"`
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
			PackageManager  string            `json:"packageManager"`
		}
		_ = json.Unmarshal(b, &pkg)

		app := NodeApp{Name: e.Name(), Path: dir, PackageName: pkg.Name, Version: pkg.Version, Description: pkg.Description,
			HasModules: exists(filepath.Join(dir, "node_modules")), Node: def}
		if nc := cfg.NodeApps[e.Name()]; nc != nil && has[nc.Node] {
			app.Node = nc.Node
		}
		for _, f := range frameworks {
			if _, ok := pkg.Dependencies[f.dep]; ok {
				app.Framework = f.name
				break
			}
			if _, ok := pkg.DevDependencies[f.dep]; ok {
				app.Framework = f.name
				break
			}
		}
		app.Manager = detectManager(dir, pkg.PackageManager)
		if r := runs[runKey(e.Name(), installKey)]; r != nil && r.running() {
			app.Installing = true
		}
		for name, command := range pkg.Scripts {
			s := NodeScript{Name: name, Command: command}
			if r := runs[runKey(e.Name(), name)]; r != nil {
				s.Running, s.URL, s.ExitCode = r.running(), r.urlValue(), r.exitCode()
			}
			app.Scripts = append(app.Scripts, s)
		}
		sort.Slice(app.Scripts, func(i, j int) bool {
			oi, iok := scriptOrder[app.Scripts[i].Name]
			oj, jok := scriptOrder[app.Scripts[j].Name]
			if iok != jok {
				return iok
			}
			if iok && oi != oj {
				return oi < oj
			}
			return app.Scripts[i].Name < app.Scripts[j].Name
		})
		apps = append(apps, app)
	}
	return apps
}

func detectManager(dir, field string) string {
	for _, pm := range []string{"pnpm", "yarn", "bun"} {
		if strings.HasPrefix(field, pm+"@") {
			return pm
		}
	}
	switch {
	case exists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(dir, "bun.lockb")), exists(filepath.Join(dir, "bun.lock")):
		return "bun"
	}
	return "npm"
}

func (m *Manager) nodeState(cfg Config) NodeState {
	installed, versions := m.installedNode()
	def := m.defaultNode(cfg, installed)
	st := NodeState{Installed: []NodeVersionState{}, Default: def, Apps: m.nodeApps(cfg)}
	for _, b := range installed {
		st.Installed = append(st.Installed, NodeVersionState{Branch: b, Version: versions[b], Default: b == def})
	}
	for _, r := range m.nodeRunsSnapshot() {
		if r.running() {
			st.Running++
		}
	}
	return st
}

func (m *Manager) SetAppNode(app, branch string) error {
	err := m.updateConfig(func(c *Config) {
		if c.NodeApps == nil {
			c.NodeApps = map[string]*NodeAppConfig{}
		}
		c.NodeApps[app] = &NodeAppConfig{Node: branch}
	})
	m.changed()
	return err
}

// ---------------- script runner ----------------

const (
	installKey = "__install"
	createKey  = "__create"
	maxLines   = 3000
)

func runKey(app, script string) string { return app + "/" + script }

type nodeRun struct {
	key  string
	node string
	proc *process

	mu    sync.Mutex
	lines []string
	url   string
	exit  *int
}

func (r *nodeRun) running() bool { return r.proc.alive() }

func (r *nodeRun) urlValue() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.url
}

func (r *nodeRun) exitCode() *int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exit
}

var (
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07`)
	urlRe  = regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1?\]):\d+[^\s'"]*`)
)

func (r *nodeRun) addLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
	if len(r.lines) > maxLines {
		r.lines = r.lines[len(r.lines)-maxLines:]
	}
	if r.url == "" {
		if u := urlRe.FindString(line); u != "" {
			u = strings.Replace(u, "0.0.0.0", "localhost", 1)
			r.url = strings.TrimRight(u, ".,;)")
		}
	}
}

func (m *Manager) nodeRunsSnapshot() map[string]*nodeRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]*nodeRun, len(m.nodeRuns))
	for k, v := range m.nodeRuns {
		out[k] = v
	}
	return out
}

// nodeEnv puts the chosen Node first on PATH and keeps dev servers from opening browsers.
func (m *Manager) nodeEnv(branch string) ([]string, error) {
	dir := m.P.Bin("node", branch)
	if branch == "" || !exists(filepath.Join(dir, "node.exe")) {
		return nil, fmt.Errorf("Node.js kurulu değil, Node.js sayfasından bir sürüm kurun")
	}
	env := []string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			env = append(env, kv)
		}
	}
	return append(env,
		"PATH="+dir+";"+os.Getenv("PATH"),
		"BROWSER=none", "FORCE_COLOR=0", "NO_UPDATE_NOTIFIER=1", "npm_config_update_notifier=false",
		"CI=true", // keeps create-* tools from asking interactive questions
	), nil
}

// managerCommand builds the shell command for running a script or installing packages.
func managerCommand(pm, script string) string {
	runner := map[string]string{"npm": "npm", "pnpm": "npx --yes pnpm", "yarn": "npx --yes yarn", "bun": "bun"}[pm]
	if script == installKey {
		return runner + " install"
	}
	return runner + " run " + script
}

// startRun launches `cmd /c <command>` in dir and streams its output to the UI.
func (m *Manager) startRun(key, node, dir, command string, env []string, onExit func(code int)) error {
	m.mu.Lock()
	if r := m.nodeRuns[key]; r != nil && r.running() {
		m.mu.Unlock()
		return fmt.Errorf("zaten çalışıyor")
	}
	m.mu.Unlock()

	cmd := hiddenCmd("cmd.exe", "/d", "/s", "/c", command)
	cmd.Dir = dir
	cmd.Env = env
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	cmd.Stdin = nil

	safe := strings.NewReplacer("/", "-", ":", "-").Replace(key)
	logFile, _ := os.OpenFile(m.P.Logs("node-"+safe+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)

	run := &nodeRun{key: key, node: node}
	run.addLine("> " + command)
	if err := cmd.Start(); err != nil {
		pw.Close()
		if logFile != nil {
			logFile.Close()
		}
		return err
	}
	run.proc = &process{cmd: cmd, done: make(chan struct{})}
	m.mu.Lock()
	m.nodeRuns[key] = run
	m.mu.Unlock()

	// Batch output into one event per ~120ms so busy dev servers don't flood the UI.
	var pending []string
	var pmu sync.Mutex
	flush := func() {
		pmu.Lock()
		batch := pending
		pending = nil
		pmu.Unlock()
		if len(batch) > 0 {
			m.emit("nodelog", map[string]any{"key": key, "lines": batch})
		}
	}
	stopTicker := make(chan struct{})
	go func() {
		t := time.NewTicker(120 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				flush()
			case <-stopTicker:
				flush()
				return
			}
		}
	}()

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := ansiRe.ReplaceAllString(sc.Text(), "")
			if i := strings.LastIndex(line, "\r"); i >= 0 { // progress bars redraw with \r
				line = line[i+1:]
			}
			if logFile != nil {
				logFile.WriteString(line + "\n")
			}
			hadURL := run.urlValue() != ""
			run.addLine(line)
			pmu.Lock()
			pending = append(pending, line)
			pmu.Unlock()
			if !hadURL && run.urlValue() != "" {
				m.changed()
			}
		}
		io.Copy(io.Discard, pr)
	}()

	go func() {
		err := cmd.Wait()
		pw.Close()
		<-readerDone
		code := 0
		if err != nil {
			code = 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		msg := fmt.Sprintf("— işlem bitti (çıkış kodu %d)", code)
		run.addLine(msg)
		pmu.Lock()
		pending = append(pending, msg)
		pmu.Unlock()
		close(stopTicker)
		run.mu.Lock()
		run.exit = &code
		run.mu.Unlock()
		if logFile != nil {
			logFile.Close()
		}
		close(run.proc.done)
		if onExit != nil {
			onExit(code)
		}
		m.changed()
	}()
	m.changed()
	return nil
}

func (m *Manager) appByName(name string) (*NodeApp, error) {
	for _, a := range m.nodeApps(m.config()) {
		if a.Name == name {
			a := a
			return &a, nil
		}
	}
	return nil, fmt.Errorf("proje bulunamadı: %s", name)
}

// RunScript starts an npm script (or "__install") for a project.
func (m *Manager) RunScript(appName, script string) error {
	app, err := m.appByName(appName)
	if err != nil {
		return err
	}
	if script != installKey {
		found := false
		for _, s := range app.Scripts {
			found = found || s.Name == script
		}
		if !found {
			return fmt.Errorf("script bulunamadı: %s", script)
		}
	}
	if app.Manager == "bun" {
		if _, err := exec.LookPath("bun"); err != nil {
			return fmt.Errorf("bu proje bun kullanıyor ama bun kurulu değil")
		}
	}
	env, err := m.nodeEnv(app.Node)
	if err != nil {
		return err
	}
	return m.startRun(runKey(appName, script), app.Node, app.Path, managerCommand(app.Manager, script), env, nil)
}

func (m *Manager) StopScript(appName, script string) {
	m.mu.Lock()
	r := m.nodeRuns[runKey(appName, script)]
	m.mu.Unlock()
	if r != nil {
		r.proc.kill()
	}
	m.changed()
}

func (m *Manager) NodeLog(appName, script string) []string {
	m.mu.Lock()
	r := m.nodeRuns[runKey(appName, script)]
	m.mu.Unlock()
	if r == nil {
		return []string{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.lines...)
}

func (m *Manager) ClearNodeLog(appName, script string) {
	m.mu.Lock()
	r := m.nodeRuns[runKey(appName, script)]
	m.mu.Unlock()
	if r != nil {
		r.mu.Lock()
		r.lines = nil
		r.mu.Unlock()
	}
}

func (m *Manager) stopAllNode() {
	for _, r := range m.nodeRunsSnapshot() {
		r.proc.kill()
	}
}

// ---------------- templates ----------------

type NodeTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

var NodeTemplates = []NodeTemplate{
	{"vite-react", "React + Vite", "TypeScript, hızlı HMR"},
	{"vite-vue", "Vue + Vite", "TypeScript, hızlı HMR"},
	{"vite-svelte", "Svelte + Vite", "TypeScript, hızlı HMR"},
	{"next", "Next.js", "React framework, App Router, TypeScript"},
	{"express", "Express API", "Minimal Node.js sunucusu"},
	{"empty", "Boş proje", "Sadece package.json"},
}

var nodeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// CreateNodeApp scaffolds a project into nodejs/<name> and installs its packages.
func (m *Manager) CreateNodeApp(name, template string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !nodeNameRe.MatchString(name) {
		return fmt.Errorf("geçersiz ad: küçük harf, rakam, nokta, tire ve alt çizgi kullanın")
	}
	dir := m.P.Node(name)
	if exists(dir) {
		return fmt.Errorf("%s zaten var", name)
	}
	cfg := m.config()
	installed, _ := m.installedNode()
	node := m.defaultNode(cfg, installed)
	env, err := m.nodeEnv(node)
	if err != nil {
		return err
	}

	install := func(code int) {
		if code == 0 && exists(filepath.Join(dir, "package.json")) && !exists(filepath.Join(dir, "node_modules")) {
			_ = m.RunScript(name, installKey)
		}
	}
	var command string
	switch template {
	case "vite-react", "vite-vue", "vite-svelte":
		tpl := strings.TrimPrefix(template, "vite-") + "-ts"
		command = fmt.Sprintf("npm create vite@latest %s -- --template %s --no-interactive", name, tpl)
	case "next":
		command = fmt.Sprintf("npx --yes create-next-app@latest %s --ts --eslint --app --src-dir --use-npm --yes", name)
	case "express", "empty":
		if err := writeStarter(dir, name, template); err != nil {
			return err
		}
		m.changed()
		if template == "express" {
			return m.RunScript(name, installKey)
		}
		return nil
	default:
		return fmt.Errorf("bilinmeyen şablon: %s", template)
	}
	return m.startRun(runKey(name, createKey), node, m.P.Node(), command, env, install)
}

func writeStarter(dir, name, template string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pkg := map[string]any{"name": name, "version": "1.0.0", "private": true}
	if template == "express" {
		pkg["type"] = "module"
		pkg["scripts"] = map[string]string{"dev": "node --watch index.js", "start": "node index.js"}
		pkg["dependencies"] = map[string]string{"express": "^5.1.0"}
		index := `import express from 'express'

const app = express()
const port = process.env.PORT || 3000

app.get('/', (req, res) => {
  res.send('<h1>` + name + ` çalışıyor 🚀</h1><p>index.js dosyasını düzenleyerek başla.</p>')
})

app.listen(port, () => {
  console.log('Sunucu hazır: http://localhost:' + port)
})
`
		if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(index), 0o644); err != nil {
			return err
		}
	} else {
		pkg["scripts"] = map[string]string{"start": "node index.js"}
		if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log('Merhaba PAMPP!')\n"), 0o644); err != nil {
			return err
		}
	}
	b, _ := json.MarshalIndent(pkg, "", "  ")
	return os.WriteFile(filepath.Join(dir, "package.json"), append(b, '\n'), 0o644)
}

// CreateLog exposes the scaffolding output of a project that is being created.
func (m *Manager) CreateLog(name string) []string { return m.NodeLog(name, createKey) }

// OpenInEditor opens a folder in VS Code when it is installed, otherwise in Explorer.
func (m *Manager) OpenInEditor(path string) error {
	rel, err := filepath.Rel(m.P.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("klasör PAMPP dışında")
	}
	if code, err := exec.LookPath("code"); err == nil {
		cmd := hiddenCmd(code, path)
		cmd.SysProcAttr.CreationFlags |= createBreakawayFromJob
		return cmd.Start()
	}
	return shellOpen(path)
}
