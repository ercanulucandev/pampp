package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Emitter func(event string, data any)

type Manager struct {
	P    Paths
	emit Emitter

	opMu sync.Mutex // serialises start/stop/install/config operations

	mu       sync.Mutex // guards the fields below
	cfg      Config
	web      *webServer
	db       *mariaDB
	pools    map[string]*phpPool
	tasks    map[string]*Task
	errors   map[string]string
	catalog  catalogCache
	ca       *localCA
	nodeRuns map[string]*nodeRun
	trusted  bool // cached: certutil is too slow to call on every State()
}

type Task struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Stage    string  `json:"stage"`
	Progress float64 `json:"progress"` // 0..1, negative = indeterminate
	Error    string  `json:"error"`
	Done     bool    `json:"done"`
}

func NewManager(root string, emit Emitter) *Manager {
	if emit == nil {
		emit = func(string, any) {}
	}
	p := Paths{Root: root}
	_ = p.Ensure()
	m := &Manager{
		P:        p,
		emit:     emit,
		cfg:      loadConfig(p.ConfigFile()),
		pools:    map[string]*phpPool{},
		tasks:    map[string]*Task{},
		errors:   map[string]string{},
		nodeRuns: map[string]*nodeRun{},
	}
	if exists(filepath.Join(p.Data("ssl"), "ca.crt")) {
		if ca, err := loadOrCreateCA(p.Data("ssl")); err == nil {
			m.ca, m.trusted = ca, ca.trusted()
		}
	}
	return m
}

// ---------------- state ----------------

type ServiceState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	Error     string `json:"error"`
}

type PHPState struct {
	Branch  string `json:"branch"`
	Version string `json:"version"`
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Default bool   `json:"default"`
	Sites   int    `json:"sites"`
}

type SiteState struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	URL     string `json:"url"`
	Path    string `json:"path"`
	DocRoot string `json:"docRoot"`
	PHP     string `json:"php"`
	System  bool   `json:"system"`
}

type ComponentState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
}

type State struct {
	Root       string           `json:"root"`
	Config     Config           `json:"config"`
	Services   []ServiceState   `json:"services"`
	PHP        []PHPState       `json:"php"`
	Sites      []SiteState      `json:"sites"`
	Components []ComponentState `json:"components"`
	Tasks      []Task           `json:"tasks"`
	VCRedist   bool             `json:"vcRedist"`
	SSL        SSLState         `json:"ssl"`
	StartURL   string           `json:"startUrl"`
	Node       NodeState        `json:"node"`
	Installed  bool             `json:"installed"` // installer copy (uninstall.exe next to the exe)
}

type SSLState struct {
	Enabled    bool   `json:"enabled"`
	Port       int    `json:"port"`
	CAExists   bool   `json:"caExists"`
	Trusted    bool   `json:"trusted"`
	Thumbprint string `json:"thumbprint"`
	Expires    string `json:"expires"`
}

func (m *Manager) dir(component string) string {
	switch component {
	case "nginx", "apache", "mariadb", "phpmyadmin", "composer":
		return m.P.Bin(component)
	}
	return ""
}

func (m *Manager) componentInstalled(id string) bool {
	switch id {
	case "nginx":
		return exists(m.P.Bin("nginx", "nginx.exe"))
	case "apache":
		return exists(m.P.Bin("apache", "bin", "httpd.exe"))
	case "mariadb":
		return exists(m.P.Bin("mariadb", "bin", "mariadbd.exe")) || exists(m.P.Bin("mariadb", "bin", "mysqld.exe"))
	case "phpmyadmin":
		return exists(m.P.Bin("phpmyadmin", "index.php"))
	case "composer":
		return exists(m.P.Bin("composer", "composer.phar"))
	}
	return false
}

// installedPHP returns installed branches (newest first) with their full versions.
func (m *Manager) installedPHP() ([]string, map[string]string) {
	versions := map[string]string{}
	entries, _ := os.ReadDir(m.P.Bin("php"))
	var branches []string
	for _, e := range entries {
		dir := m.P.Bin("php", e.Name())
		if e.IsDir() && exists(filepath.Join(dir, "php-cgi.exe")) && !strings.Contains(e.Name(), ".new") && !strings.Contains(e.Name(), ".old") {
			branches = append(branches, e.Name())
			versions[e.Name()] = readVersionFile(dir)
		}
	}
	sort.Slice(branches, func(i, j int) bool { return compareVersions(branches[i], branches[j]) > 0 })
	return branches, versions
}

// defaultPHP resolves the configured default, falling back to the newest installed branch.
func (m *Manager) defaultPHP(cfg Config, installed []string) string {
	for _, b := range installed {
		if b == cfg.DefaultPHP {
			return b
		}
	}
	if len(installed) > 0 {
		return installed[0]
	}
	return ""
}

var siteNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// siteURL prefers HTTPS when it is enabled and omits default ports.
func siteURL(cfg Config, domain string) string {
	scheme, port, def := "http", cfg.HTTPPort, 80
	if cfg.SSL {
		scheme, port, def = "https", cfg.HTTPSPort, 443
	}
	if port == def {
		return scheme + "://" + domain
	}
	return fmt.Sprintf("%s://%s:%d", scheme, domain, port)
}

func (m *Manager) sites(cfg Config) []SiteState {
	installed, _ := m.installedPHP()
	def := m.defaultPHP(cfg, installed)
	has := map[string]bool{}
	for _, b := range installed {
		has[b] = true
	}
	phpFor := func(name string) string {
		if sc, ok := cfg.Sites[name]; ok && has[sc.PHP] {
			return sc.PHP
		}
		return def
	}
	url := func(domain string) string { return siteURL(cfg, domain) }

	out := []SiteState{}
	entries, _ := os.ReadDir(m.P.WWW())
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if !e.IsDir() || !siteNameRe.MatchString(name) || name == "phpmyadmin" {
			continue
		}
		path := m.P.WWW(e.Name())
		docRoot := path
		for _, sub := range []string{"public", "web", "public_html"} {
			if exists(filepath.Join(path, sub)) {
				docRoot = filepath.Join(path, sub)
				break
			}
		}
		domain := name + "." + cfg.TLD
		out = append(out, SiteState{Name: name, Domain: domain, URL: url(domain), Path: path, DocRoot: docRoot, PHP: phpFor(name)})
	}
	if m.componentInstalled("phpmyadmin") {
		domain := "phpmyadmin." + cfg.TLD
		dir := m.P.Bin("phpmyadmin")
		out = append(out, SiteState{Name: "phpmyadmin", Domain: domain, URL: url(domain), Path: dir, DocRoot: dir, PHP: phpFor("phpmyadmin"), System: true})
	}
	return out
}

func (m *Manager) State() State {
	m.mu.Lock()
	cfg := m.cfg
	web, db := m.web, m.db
	pools := map[string]*phpPool{}
	for k, v := range m.pools {
		pools[k] = v
	}
	errs := map[string]string{}
	for k, v := range m.errors {
		errs[k] = v
	}
	tasks := []Task{}
	for _, t := range m.tasks {
		tasks = append(tasks, *t)
	}
	ssl := SSLState{Enabled: cfg.SSL, Port: cfg.HTTPSPort, Trusted: m.trusted}
	if m.ca != nil {
		ssl.CAExists = true
		ssl.Thumbprint = m.ca.thumbprint()
		ssl.Expires = m.ca.cert.NotAfter.Format("2006-01-02")
	}
	m.mu.Unlock()

	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	installed, versions := m.installedPHP()
	def := m.defaultPHP(cfg, installed)
	sites := m.sites(cfg)

	webName := "Nginx"
	if cfg.WebServer == "apache" {
		webName = "Apache"
	}
	st := State{
		Root:      m.P.Root,
		PHP:       []PHPState{},
		Config:    cfg,
		Sites:     sites,
		Tasks:     tasks,
		VCRedist:  vcRedistInstalled(),
		SSL:       ssl,
		StartURL:  siteURL(cfg, "localhost"),
		Node:      m.nodeState(cfg),
		Installed: installedCopy(),
		Services: []ServiceState{
			{ID: "web", Name: webName, Installed: m.componentInstalled(cfg.WebServer), Version: readVersionFile(m.dir(cfg.WebServer)),
				Running: web != nil && web.proc.alive(), Port: cfg.HTTPPort, Error: errs["web"]},
			{ID: "db", Name: "MariaDB", Installed: m.componentInstalled("mariadb"), Version: readVersionFile(m.dir("mariadb")),
				Running: db != nil && db.proc.alive(), Port: cfg.DBPort, Error: errs["db"]},
		},
	}
	for _, b := range installed {
		count := 0
		for _, s := range sites {
			if s.PHP == b {
				count++
			}
		}
		pool := pools[b]
		st.PHP = append(st.PHP, PHPState{Branch: b, Version: versions[b], Running: pool != nil && pool.running(),
			Port: phpPort(b, 0), Default: b == def, Sites: count})
	}
	for _, c := range []struct{ id, name string }{{"nginx", "Nginx"}, {"apache", "Apache"}, {"mariadb", "MariaDB"}, {"phpmyadmin", "phpMyAdmin"}, {"composer", "Composer"}} {
		st.Components = append(st.Components, ComponentState{ID: c.id, Name: c.name, Installed: m.componentInstalled(c.id), Version: readVersionFile(m.dir(c.id))})
	}
	return st
}

func (m *Manager) changed() { m.emit("state", m.State()) }

func (m *Manager) setError(id string, err error) {
	m.mu.Lock()
	if err != nil {
		m.errors[id] = err.Error()
	} else {
		delete(m.errors, id)
	}
	m.mu.Unlock()
}

func (m *Manager) config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) updateConfig(fn func(c *Config)) error {
	m.mu.Lock()
	sites := map[string]*SiteConfig{}
	for k, v := range m.cfg.Sites {
		cp := *v
		sites[k] = &cp
	}
	m.cfg.Sites = sites
	apps := map[string]*NodeAppConfig{}
	for k, v := range m.cfg.NodeApps {
		cp := *v
		apps[k] = &cp
	}
	m.cfg.NodeApps = apps
	fn(&m.cfg)
	m.cfg.normalize()
	cfg := m.cfg
	m.mu.Unlock()
	return saveConfig(m.P.ConfigFile(), cfg)
}

// ---------------- services ----------------

func (m *Manager) Catalog(force bool) *Catalog { return m.catalog.get(force) }

func (m *Manager) webRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.web != nil && m.web.proc.alive()
}

func (m *Manager) dbRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.db != nil && m.db.proc.alive()
}

func (m *Manager) StartAll() error {
	errWeb := m.StartService("web")
	errDB := m.StartService("db")
	if errWeb != nil {
		return errWeb
	}
	return errDB
}

func (m *Manager) StopAll() {
	m.StopService("web")
	m.StopService("db")
}

func (m *Manager) StartService(id string) error {
	m.opMu.Lock()
	var err error
	switch id {
	case "web":
		err = m.startWeb()
	case "db":
		err = m.startDB()
	default:
		err = fmt.Errorf("bilinmeyen servis: %s", id)
	}
	m.setError(id, err)
	m.opMu.Unlock()
	m.changed()
	return err
}

func (m *Manager) StopService(id string) {
	m.opMu.Lock()
	switch id {
	case "web":
		m.stopWeb()
	case "db":
		m.mu.Lock()
		db := m.db
		m.mu.Unlock()
		if db != nil {
			db.stop()
		}
	}
	m.setError(id, nil)
	m.opMu.Unlock()
	m.changed()
}

func (m *Manager) RestartService(id string) error {
	m.StopService(id)
	return m.StartService(id)
}

func (m *Manager) startDB() error {
	if m.dbRunning() {
		return nil
	}
	if !m.componentInstalled("mariadb") {
		return fmt.Errorf("MariaDB kurulu değil, Bileşenler sayfasından kurun")
	}
	cfg := m.config()
	db := &mariaDB{dir: m.P.Bin("mariadb"), dataDir: m.P.Data("mariadb"), conf: m.P.Etc("mariadb", "my.ini"), port: cfg.DBPort, password: cfg.DBPassword}
	if err := db.writeConfig(m.P.Logs()); err != nil {
		return err
	}
	if err := db.start(m.P.Logs("mariadb.log")); err != nil {
		return err
	}
	m.mu.Lock()
	m.db = db
	m.mu.Unlock()
	return nil
}

// neededPHP lists the PHP branches that must run for the current sites.
func (m *Manager) neededPHP(cfg Config) []string {
	installed, _ := m.installedPHP()
	set := map[string]bool{}
	if def := m.defaultPHP(cfg, installed); def != "" {
		set[def] = true
	}
	for _, s := range m.sites(cfg) {
		if s.PHP != "" {
			set[s.PHP] = true
		}
	}
	return sortedBranches(set)
}

// syncPools starts missing PHP pools and stops the ones no site uses anymore.
func (m *Manager) syncPools(cfg Config) ([]string, error) {
	needed := m.neededPHP(cfg)
	want := map[string]bool{}
	for _, b := range needed {
		want[b] = true
	}
	m.mu.Lock()
	pools := m.pools
	m.mu.Unlock()
	for b, pool := range pools {
		if !want[b] || pool.workers != cfg.PHPWorkers {
			pool.stop()
			m.mu.Lock()
			delete(m.pools, b)
			m.mu.Unlock()
		}
	}
	var running []string
	var firstErr error
	for _, b := range needed {
		m.mu.Lock()
		pool := m.pools[b]
		m.mu.Unlock()
		if pool != nil && pool.running() {
			running = append(running, b)
			continue
		}
		if pool != nil {
			pool.stop()
		}
		pool = newPHPPool(b, m.P.Bin("php", b), cfg.PHPWorkers, m.P.Logs("php-"+b+".log"))
		if err := pool.start(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		m.mu.Lock()
		m.pools[b] = pool
		m.mu.Unlock()
		running = append(running, b)
	}
	return running, firstErr
}

func (m *Manager) webConfig(cfg Config, running []string) webConfig {
	installed, _ := m.installedPHP()
	isRunning := map[string]bool{}
	for _, b := range running {
		isRunning[b] = true
	}
	phpOrEmpty := func(b string) string {
		if isRunning[b] {
			return b
		}
		return ""
	}
	wc := webConfig{
		Port: cfg.HTTPPort, LocalRoot: m.P.Data("start"), Logs: m.P.Logs(), Tmp: m.P.Tmp(),
		DefaultPHP: phpOrEmpty(m.defaultPHP(cfg, installed)), PHPs: running, Workers: cfg.PHPWorkers,
		SSL: cfg.SSL, SSLPort: cfg.HTTPSPort,
	}
	for _, s := range m.sites(cfg) {
		wc.Hosts = append(wc.Hosts, vhost{Name: s.Name, Domain: s.Domain, DocRoot: s.DocRoot, PHP: phpOrEmpty(s.PHP)})
	}
	return wc
}

// ensureCertificate makes sure the CA exists and the site certificate covers every domain.
func (m *Manager) ensureCertificate(wc *webConfig) error {
	m.mu.Lock()
	ca := m.ca
	m.mu.Unlock()
	if ca == nil {
		var err error
		if ca, err = loadOrCreateCA(m.P.Data("ssl")); err != nil {
			return fmt.Errorf("sertifika otoritesi oluşturulamadı: %w", err)
		}
		trusted := ca.trusted()
		m.mu.Lock()
		m.ca, m.trusted = ca, trusted
		m.mu.Unlock()
	}
	var domains []string
	for _, h := range wc.Hosts {
		domains = append(domains, h.Domain)
	}
	cert, key, err := ca.siteCertificate(domains)
	if err != nil {
		return fmt.Errorf("site sertifikası üretilemedi: %w", err)
	}
	wc.CertFile, wc.KeyFile = cert, key
	ca.appendToBundle(m.P.Bin("ssl", "cacert.pem"))
	return nil
}

func (m *Manager) writeWebConfig(cfg Config, running []string) (*webServer, error) {
	ws := &webServer{kind: cfg.WebServer, dir: m.P.Bin(cfg.WebServer), port: cfg.HTTPPort}
	wc := m.webConfig(cfg, running)
	if wc.SSL {
		if err := m.ensureCertificate(&wc); err != nil {
			return nil, err
		}
	}
	if err := m.writeStartPage(cfg, wc); err != nil {
		return nil, err
	}
	var err error
	if ws.kind == "apache" {
		ws.conf, err = writeApacheConfig(m.P, ws.dir, wc)
	} else {
		ws.conf, err = writeNginxConfig(m.P, ws.dir, wc)
	}
	return ws, err
}

func (m *Manager) startWeb() error {
	if m.webRunning() {
		return nil
	}
	cfg := m.config()
	if !m.componentInstalled(cfg.WebServer) {
		return fmt.Errorf("%s kurulu değil, Bileşenler sayfasından kurun", cfg.WebServer)
	}
	if !portFree(cfg.HTTPPort) {
		return portBusyError(cfg.HTTPPort)
	}
	if cfg.SSL && !portFree(cfg.HTTPSPort) {
		return portBusyError(cfg.HTTPSPort)
	}
	running, poolErr := m.syncPools(cfg)
	ws, err := m.writeWebConfig(cfg, running)
	if err != nil {
		return err
	}
	if err := ws.start(m.P.Logs(cfg.WebServer + ".log")); err != nil {
		m.stopPools()
		return err
	}
	m.mu.Lock()
	m.web = ws
	m.mu.Unlock()
	m.syncHosts(cfg)
	return poolErr
}

func (m *Manager) stopPools() {
	m.mu.Lock()
	pools := m.pools
	m.pools = map[string]*phpPool{}
	m.mu.Unlock()
	for _, p := range pools {
		p.stop()
	}
}

func (m *Manager) stopWeb() {
	m.mu.Lock()
	ws := m.web
	m.web = nil
	m.mu.Unlock()
	if ws != nil {
		ws.stop()
	}
	m.stopPools()
}

// apply regenerates configs after a site/PHP change and reloads the running server.
func (m *Manager) apply() error {
	cfg := m.config()
	m.syncHosts(cfg)
	if !m.webRunning() {
		return nil
	}
	running, poolErr := m.syncPools(cfg)
	ws, err := m.writeWebConfig(cfg, running)
	if err != nil {
		return err
	}
	m.mu.Lock()
	current := m.web
	m.mu.Unlock()
	ws.proc = current.proc
	if current.kind != ws.kind {
		current.stop()
		ws.proc = nil
	}
	err = ws.reload(m.P.Logs(cfg.WebServer + ".log"))
	m.mu.Lock()
	m.web = ws
	m.mu.Unlock()
	if err != nil {
		return err
	}
	return poolErr
}

func (m *Manager) syncHosts(cfg Config) {
	if cfg.TLD == "localhost" {
		return
	}
	var domains []string
	for _, s := range m.sites(cfg) {
		domains = append(domains, s.Domain, "www."+s.Domain)
	}
	if hostsUpToDate(domains) {
		return
	}
	list := m.P.Tmp("hosts.txt")
	if err := os.WriteFile(list, []byte(strings.Join(domains, "\n")), 0o644); err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = runElevated(exe, fmt.Sprintf(`--sync-hosts "%s"`, list))
}

func (m *Manager) Shutdown() {
	m.stopAllNode()
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.stopWeb()
	m.mu.Lock()
	db := m.db
	m.mu.Unlock()
	if db != nil {
		db.stop()
	}
}

// ---------------- sites & settings ----------------

func (m *Manager) SetSitePHP(site, branch string) error {
	m.opMu.Lock()
	err := m.updateConfig(func(c *Config) {
		if c.Sites[site] == nil {
			c.Sites[site] = &SiteConfig{}
		}
		c.Sites[site].PHP = branch
	})
	if err == nil {
		err = m.apply()
	}
	m.setError("web", err)
	m.opMu.Unlock()
	m.changed()
	return err
}

func (m *Manager) SetDefaultPHP(branch string) error {
	m.opMu.Lock()
	err := m.updateConfig(func(c *Config) { c.DefaultPHP = branch })
	if err == nil {
		err = m.apply()
	}
	m.setError("web", err)
	m.opMu.Unlock()
	m.changed()
	return err
}

// CreateSite makes www/<name> with a welcome page and returns the new site.
func (m *Manager) CreateSite(name, php string) (*SiteState, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !siteNameRe.MatchString(name) || name == "phpmyadmin" {
		return nil, fmt.Errorf("geçersiz ad: sadece küçük harf, rakam ve tire kullanın")
	}
	dir := m.P.WWW(name)
	if exists(dir) {
		return nil, fmt.Errorf("%s zaten var", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.php"), []byte(siteWelcomePage(name, m.config().DBPort, m.config().DBPassword)), 0o644); err != nil {
		return nil, err
	}
	m.opMu.Lock()
	if php != "" {
		_ = m.updateConfig(func(c *Config) { c.Sites[name] = &SiteConfig{PHP: php} })
	}
	err := m.apply()
	m.opMu.Unlock()
	m.changed()
	for _, s := range m.sites(m.config()) {
		if s.Name == name {
			return &s, err
		}
	}
	return nil, err
}

func (m *Manager) TrustCA() error {
	m.mu.Lock()
	ca := m.ca
	m.mu.Unlock()
	var err error
	if ca == nil {
		if ca, err = loadOrCreateCA(m.P.Data("ssl")); err != nil {
			return err
		}
	}
	err = ca.trust()
	m.mu.Lock()
	m.ca, m.trusted = ca, ca.trusted()
	m.mu.Unlock()
	m.changed()
	return err
}

func (m *Manager) UntrustCA() error {
	m.mu.Lock()
	ca := m.ca
	m.mu.Unlock()
	if ca == nil {
		return nil
	}
	err := ca.untrust()
	m.mu.Lock()
	m.trusted = ca.trusted()
	m.mu.Unlock()
	m.changed()
	return err
}

func (m *Manager) SaveSettings(next Config) error {
	m.opMu.Lock()
	prev := m.config()
	err := m.updateConfig(func(c *Config) {
		c.WebServer = next.WebServer
		c.HTTPPort = next.HTTPPort
		c.SSL = next.SSL
		c.HTTPSPort = next.HTTPSPort
		c.DBPort = next.DBPort
		c.TLD = strings.ToLower(strings.Trim(next.TLD, ". "))
		c.PHPWorkers = next.PHPWorkers
		c.AutoStart = next.AutoStart
		c.CloseToTray = next.CloseToTray
	})
	cfg := m.config()
	m.opMu.Unlock()
	if err != nil {
		return err
	}
	if m.webRunning() && (prev.WebServer != cfg.WebServer || prev.HTTPPort != cfg.HTTPPort ||
		prev.SSL != cfg.SSL || prev.HTTPSPort != cfg.HTTPSPort) {
		m.StopService("web")
		err = m.StartService("web")
	} else if prev.TLD != cfg.TLD || prev.PHPWorkers != cfg.PHPWorkers {
		m.opMu.Lock()
		err = m.apply()
		m.opMu.Unlock()
	}
	if prev.DBPort != cfg.DBPort && m.componentInstalled("phpmyadmin") {
		_ = writePMAConfig(m.P.Bin("phpmyadmin"), cfg.DBPort, cfg.DBPassword)
	}
	if m.dbRunning() && prev.DBPort != cfg.DBPort {
		m.StopService("db")
		if e := m.StartService("db"); err == nil {
			err = e
		}
	}
	m.changed()
	return err
}

func (m *Manager) CloseToTray() bool { return m.config().CloseToTray }
func (m *Manager) AutoStart() bool   { return m.config().AutoStart }

// ---------------- PHP extensions ----------------

func (m *Manager) phpDir(branch string) (string, error) {
	dir := m.P.Bin("php", branch)
	if !siteNameRe.MatchString(strings.ReplaceAll(branch, ".", "-")) || !exists(filepath.Join(dir, "php-cgi.exe")) {
		return "", fmt.Errorf("PHP %s kurulu değil", branch)
	}
	return dir, nil
}

func (m *Manager) Extensions(branch string) ([]PHPExtension, error) {
	dir, err := m.phpDir(branch)
	if err != nil {
		return nil, err
	}
	return listExtensions(dir)
}

func (m *Manager) SetExtension(branch, name string, on bool) error {
	dir, err := m.phpDir(branch)
	if err != nil {
		return err
	}
	if err := setExtension(dir, name, on); err != nil {
		return err
	}
	return m.RestartPHP(branch)
}

func (m *Manager) RestartPHP(branch string) error {
	m.opMu.Lock()
	m.mu.Lock()
	pool := m.pools[branch]
	m.mu.Unlock()
	var err error
	if pool != nil {
		pool.stop()
		err = pool.start()
	}
	m.opMu.Unlock()
	m.changed()
	return err
}

func (m *Manager) PHPIniPath(branch string) (string, error) {
	dir, err := m.phpDir(branch)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "php.ini"), nil
}

func (m *Manager) RemovePHP(branch string) error {
	dir, err := m.phpDir(branch)
	if err != nil {
		return err
	}
	m.opMu.Lock()
	defer func() { m.opMu.Unlock(); m.changed() }()
	m.mu.Lock()
	pool := m.pools[branch]
	delete(m.pools, branch)
	m.mu.Unlock()
	if pool != nil {
		pool.stop()
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	_ = m.updateConfig(func(c *Config) {
		for _, s := range c.Sites {
			if s.PHP == branch {
				s.PHP = ""
			}
		}
	})
	return m.apply()
}

// ---------------- logs ----------------

type LogFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (m *Manager) Logs() []LogFile {
	entries, _ := os.ReadDir(m.P.Logs())
	out := []LogFile{}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() {
			out = append(out, LogFile{Name: e.Name(), Size: info.Size()})
		}
	}
	return out
}

func (m *Manager) ReadLog(name string, maxBytes int64) (string, error) {
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("geçersiz dosya")
	}
	f, err := os.Open(m.P.Logs(name))
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, _ := f.Stat()
	if info.Size() > maxBytes {
		f.Seek(-maxBytes, 2)
	}
	buf := make([]byte, maxBytes)
	n, _ := f.Read(buf)
	return string(buf[:n]), nil
}

func (m *Manager) ClearLog(name string) error {
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("geçersiz dosya")
	}
	return os.Truncate(m.P.Logs(name), 0)
}

// ---------------- shell helpers ----------------

func (m *Manager) Open(target string) error { return shellOpen(target) }

func (m *Manager) OpenTerminal(dir string) error {
	if dir == "" {
		dir = m.P.WWW()
	}
	cfg := m.config()
	installed, _ := m.installedPHP()
	var extra []string
	if def := m.defaultPHP(cfg, installed); def != "" {
		extra = append(extra, m.P.Bin("php", def))
	}
	if nodes, _ := m.installedNode(); len(nodes) > 0 {
		extra = append(extra, m.P.Bin("node", m.defaultNode(cfg, nodes)))
	}
	extra = append(extra, m.P.Bin("composer"), m.P.Bin("mariadb", "bin"))
	env := append(os.Environ(), "PATH="+strings.Join(extra, ";")+";"+os.Getenv("PATH"))
	return openConsole(dir, env, "PAMPP", "PAMPP terminali - php, node, composer ve mariadb hazir.")
}

// ---------------- installs ----------------

func (m *Manager) newTask(label string) *Task {
	t := &Task{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Label: label, Progress: -1}
	m.mu.Lock()
	m.tasks[t.ID] = t
	m.mu.Unlock()
	m.emit("task", *t)
	return t
}

func (m *Manager) updateTask(t *Task, fn func(t *Task)) {
	m.mu.Lock()
	fn(t)
	snapshot := *t
	m.mu.Unlock()
	m.emit("task", snapshot)
}

func (m *Manager) finishTask(t *Task, err error) {
	m.updateTask(t, func(t *Task) {
		t.Done = true
		t.Progress = 1
		if err != nil {
			t.Error = err.Error()
		}
	})
	go func() {
		time.Sleep(8 * time.Second)
		m.mu.Lock()
		delete(m.tasks, t.ID)
		m.mu.Unlock()
		m.changed()
	}()
	m.changed()
}

func (m *Manager) DismissTask(id string) {
	m.mu.Lock()
	delete(m.tasks, id)
	m.mu.Unlock()
	m.changed()
}

// Install downloads and installs a component in the background and returns the task id.
// component is php|nginx|apache|mariadb|phpmyadmin|composer; branch is only used for php.
func (m *Manager) Install(component, branch string) string {
	label := map[string]string{"php": "PHP " + branch, "nginx": "Nginx", "apache": "Apache", "mariadb": "MariaDB", "phpmyadmin": "phpMyAdmin", "composer": "Composer", "node": "Node.js " + branch}[component]
	t := m.newTask(label + " kuruluyor")
	go func() {
		err := m.install(t, component, branch)
		if err == nil {
			m.updateTask(t, func(t *Task) { t.Label = label + " kuruldu" })
		}
		m.finishTask(t, err)
	}()
	return t.ID
}

// QuickSetup installs a complete stack: nginx, two PHP versions, MariaDB, phpMyAdmin and Composer.
func (m *Manager) QuickSetup() string {
	t := m.newTask("Hızlı kurulum")
	go func() {
		err := func() error {
			cat := m.Catalog(false)
			if len(cat.PHP) == 0 {
				return fmt.Errorf("sürüm listesi alınamadı: %s", strings.Join(cat.Errors, "; "))
			}
			steps := [][2]string{{"nginx", ""}, {"mariadb", ""}, {"phpmyadmin", ""}, {"composer", ""}}
			var phps []string
			for _, r := range cat.PHP {
				if compareVersions(r.Branch, "8.3") >= 0 && len(phps) < 2 {
					phps = append(phps, r.Branch)
				}
			}
			for i := len(phps) - 1; i >= 0; i-- {
				steps = append([][2]string{{"php", phps[i]}}, steps...)
			}
			// Newest LTS Node.js for the nodejs/ projects.
			for _, r := range cat.Node {
				if r.LTS != "" {
					if nodes, _ := m.installedNode(); len(nodes) == 0 {
						steps = append(steps, [2]string{"node", r.Branch})
					}
					break
				}
			}
			for _, s := range steps {
				if s[0] != "php" && s[0] != "node" && m.componentInstalled(s[0]) {
					continue
				}
				if s[0] == "php" && exists(m.P.Bin("php", s[1], "php-cgi.exe")) {
					continue
				}
				if err := m.install(t, s[0], s[1]); err != nil {
					return err
				}
			}
			if len(phps) > 0 {
				_ = m.updateConfig(func(c *Config) {
					if c.DefaultPHP == "" {
						c.DefaultPHP = phps[0]
					}
				})
			}
			m.updateTask(t, func(t *Task) { t.Stage = "Servisler başlatılıyor"; t.Progress = -1 })
			return m.StartAll()
		}()
		if err == nil {
			m.updateTask(t, func(t *Task) { t.Label = "Kurulum tamamlandı"; t.Stage = "" })
		}
		m.finishTask(t, err)
	}()
	return t.ID
}

func (m *Manager) findRelease(component, branch string) (*Release, error) {
	cat := m.Catalog(false)
	var r *Release
	switch component {
	case "php":
		for i := range cat.PHP {
			if cat.PHP[i].Branch == branch {
				r = &cat.PHP[i]
			}
		}
	case "nginx":
		r = cat.Nginx
	case "apache":
		r = cat.Apache
	case "mariadb":
		r = cat.MariaDB
	case "phpmyadmin":
		r = cat.PhpMyAdmin
	case "composer":
		r = cat.Composer
	}
	if r == nil {
		if len(cat.Errors) > 0 {
			return nil, fmt.Errorf("sürüm bilgisi alınamadı: %s", strings.Join(cat.Errors, "; "))
		}
		return nil, fmt.Errorf("%s %s bulunamadı", component, branch)
	}
	return r, nil
}

func (m *Manager) fetch(t *Task, name, url, fallback, dest string) error {
	m.updateTask(t, func(t *Task) { t.Stage = name + " indiriliyor"; t.Progress = 0 })
	report := func(done, total int64) {
		m.updateTask(t, func(t *Task) {
			if total > 0 {
				t.Progress = float64(done) / float64(total)
			} else {
				t.Progress = -1
			}
		})
	}
	err := download(url, dest, report)
	if err != nil && fallback != "" {
		err = download(fallback, dest, report)
	}
	return err
}

func (m *Manager) install(t *Task, component, branch string) error {
	if component == "node" {
		return m.installNode(t, branch)
	}
	rel, err := m.findRelease(component, branch)
	if err != nil {
		return err
	}
	name := map[string]string{"php": "PHP " + rel.Version, "nginx": "Nginx " + rel.Version, "apache": "Apache " + rel.Version,
		"mariadb": "MariaDB " + rel.Version, "phpmyadmin": "phpMyAdmin " + rel.Version, "composer": "Composer " + rel.Version}[component]

	if component == "composer" {
		dir := m.P.Bin("composer")
		os.MkdirAll(dir, 0o755)
		if err := m.fetch(t, name, rel.URL, "", filepath.Join(dir, "composer.phar")); err != nil {
			return err
		}
		bat := "@echo off\r\nphp \"%~dp0composer.phar\" %*\r\n"
		os.WriteFile(filepath.Join(dir, "composer.bat"), []byte(bat), 0o644)
		return writeVersionFile(dir, rel.Version)
	}

	zipPath := m.P.Tmp("downloads", filepath.Base(rel.URL))
	if err := m.fetch(t, name, rel.URL, rel.Fallback, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)
	m.updateTask(t, func(t *Task) { t.Stage = name + " kuruluyor"; t.Progress = -1 })

	m.opMu.Lock()
	defer m.opMu.Unlock()
	cfg := m.config()

	switch component {
	case "php":
		dir := m.P.Bin("php", rel.Branch)
		m.mu.Lock()
		pool := m.pools[rel.Branch]
		m.mu.Unlock()
		if pool != nil {
			pool.stop()
		}
		if err := installArchive(zipPath, dir, "php-cgi.exe", []string{"php.ini"}); err != nil {
			return err
		}
		writeVersionFile(dir, rel.Version)
		ca := m.ensureCABundle()
		if err := preparePHPIni(dir, ca); err != nil {
			return err
		}
		if pool != nil {
			pool.start()
		}
		if cfg.DefaultPHP == "" {
			m.updateConfig(func(c *Config) { c.DefaultPHP = rel.Branch })
		}
		return m.apply()

	case "nginx", "apache":
		wasRunning := m.webRunning() && cfg.WebServer == component
		if wasRunning {
			m.stopWeb()
		}
		marker := map[string]string{"nginx": "nginx.exe", "apache": filepath.Join("bin", "httpd.exe")}[component]
		dir := m.P.Bin(component)
		if err := installArchive(zipPath, dir, marker, nil); err != nil {
			return err
		}
		writeVersionFile(dir, rel.Version)
		if wasRunning {
			return m.startWeb()
		}
		return nil

	case "mariadb":
		wasRunning := m.dbRunning()
		if wasRunning {
			m.mu.Lock()
			db := m.db
			m.mu.Unlock()
			db.stop()
		}
		dir := m.P.Bin("mariadb")
		if err := installArchive(zipPath, dir, filepath.Join("bin", "mariadb-admin.exe"), nil); err != nil {
			if err2 := installArchive(zipPath, dir, filepath.Join("bin", "mysqladmin.exe"), nil); err2 != nil {
				return err
			}
		}
		writeVersionFile(dir, rel.Version)
		if wasRunning {
			return m.startDB()
		}
		return nil

	case "phpmyadmin":
		dir := m.P.Bin("phpmyadmin")
		if err := installArchive(zipPath, dir, "index.php", nil); err != nil {
			return err
		}
		writeVersionFile(dir, rel.Version)
		if err := writePMAConfig(dir, cfg.DBPort, cfg.DBPassword); err != nil {
			return err
		}
		// phpMyAdmin 5.2 officially supports PHP < 8.4; pick a compatible installed branch if any.
		if cfg.Sites["phpmyadmin"] == nil {
			installed, _ := m.installedPHP()
			for _, b := range installed {
				if compareVersions(b, "8.4") < 0 {
					m.updateConfig(func(c *Config) { c.Sites["phpmyadmin"] = &SiteConfig{PHP: b} })
					break
				}
			}
		}
		return m.apply()
	}
	return fmt.Errorf("bilinmeyen bileşen: %s", component)
}

// ensureCABundle downloads Mozilla's CA bundle so PHP's curl/openssl can verify HTTPS.
func (m *Manager) ensureCABundle() string {
	path := m.P.Bin("ssl", "cacert.pem")
	if exists(path) {
		return path
	}
	if err := download("https://curl.se/ca/cacert.pem", path, nil); err != nil {
		return ""
	}
	return path
}

func writePMAConfig(dir string, port int, password string) error {
	secret := make([]byte, 16)
	rand.Read(secret)
	os.MkdirAll(filepath.Join(dir, "tmp"), 0o755)
	conf := fmt.Sprintf(`<?php
// Generated by PAMPP
$cfg['blowfish_secret'] = '%s';
$i = 1;
$cfg['Servers'][$i]['auth_type'] = 'config';
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['port'] = '%d';
$cfg['Servers'][$i]['user'] = 'root';
$cfg['Servers'][$i]['password'] = '%s';
$cfg['Servers'][$i]['AllowNoPassword'] = true;
$cfg['TempDir'] = __DIR__ . '/tmp';
$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
$cfg['DefaultLang'] = 'tr';
$cfg['ThemeDefault'] = 'pmahomme';
$cfg['VersionCheck'] = false;
`, hex.EncodeToString(secret), port, phpString(password))
	return os.WriteFile(filepath.Join(dir, "config.inc.php"), []byte(conf), 0o644)
}

// installedCopy reports whether the running executable came from the installer.
func installedCopy() bool {
	exe, err := os.Executable()
	return err == nil && IsInstalled(filepath.Dir(exe))
}
