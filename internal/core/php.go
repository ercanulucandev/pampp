package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// phpPort returns the first FastCGI port for a PHP branch: 8.3 -> 10830.
func phpPort(branch string, worker int) int {
	parts := strings.SplitN(branch, ".", 2)
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return 10000 + (major*10+minor)*10 + worker
}

func phpUpstream(branch string) string { return "php" + strings.ReplaceAll(branch, ".", "") }

// phpPool keeps N php-cgi workers alive for one PHP branch.
type phpPool struct {
	branch  string
	dir     string
	workers int
	logPath string

	mu      sync.Mutex
	procs   []*process
	stopped bool
	stopCh  chan struct{}
}

func newPHPPool(branch, dir string, workers int, logPath string) *phpPool {
	return &phpPool{branch: branch, dir: dir, workers: workers, logPath: logPath}
}

func (p *phpPool) start() error {
	exe := filepath.Join(p.dir, "php-cgi.exe")
	if !exists(exe) {
		return fmt.Errorf("PHP %s kurulu değil", p.branch)
	}
	for i := 0; i < p.workers; i++ {
		if port := phpPort(p.branch, i); !portFree(port) {
			return portBusyError(port)
		}
	}
	p.mu.Lock()
	p.stopped = false
	p.stopCh = make(chan struct{})
	p.procs = make([]*process, p.workers)
	p.mu.Unlock()
	for i := 0; i < p.workers; i++ {
		if err := p.spawn(i); err != nil {
			p.stop()
			return err
		}
		go p.supervise(i)
	}
	if !waitPort(phpPort(p.branch, 0), 5*time.Second) {
		p.stop()
		return fmt.Errorf("PHP %s başlatılamadı, logs/php-%s.log dosyasına bakın", p.branch, p.branch)
	}
	return nil
}

func (p *phpPool) spawn(i int) error {
	cmd := hiddenCmd(filepath.Join(p.dir, "php-cgi.exe"),
		"-b", fmt.Sprintf("127.0.0.1:%d", phpPort(p.branch, i)),
		"-c", filepath.Join(p.dir, "php.ini"))
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), "PHP_FCGI_MAX_REQUESTS=5000", "PATH="+p.dir+";"+os.Getenv("PATH"))
	proc, err := startProcess(cmd, p.logPath)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.procs[i] = proc
	p.mu.Unlock()
	return nil
}

// supervise restarts a worker whenever it exits (php-cgi recycles itself after N requests).
func (p *phpPool) supervise(i int) {
	for {
		p.mu.Lock()
		proc, stopCh := p.procs[i], p.stopCh
		p.mu.Unlock()
		select {
		case <-stopCh:
			return
		case <-proc.done:
		}
		p.mu.Lock()
		stopped := p.stopped
		p.mu.Unlock()
		if stopped {
			return
		}
		time.Sleep(200 * time.Millisecond)
		if err := p.spawn(i); err != nil {
			time.Sleep(2 * time.Second)
		}
	}
}

func (p *phpPool) running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return false
	}
	for _, pr := range p.procs {
		if pr.alive() {
			return true
		}
	}
	return false
}

func (p *phpPool) stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	if p.stopCh != nil {
		close(p.stopCh)
	}
	procs := p.procs
	p.mu.Unlock()
	for _, pr := range procs {
		pr.kill()
	}
}

// ---- php.ini handling ----

var defaultExtensions = []string{"curl", "exif", "fileinfo", "gd", "gd2", "gettext", "intl", "mbstring", "mysqli", "openssl", "pdo_mysql", "pdo_sqlite", "sqlite3", "sodium", "zip"}

var iniExtRe = regexp.MustCompile(`^\s*(;?)\s*extension\s*=\s*"?(?:php_)?([A-Za-z0-9_]+?)(?:\.dll)?"?\s*$`)

// preparePHPIni creates php.ini from php.ini-development with sensible local defaults.
func preparePHPIni(dir, caFile string) error {
	ini := filepath.Join(dir, "php.ini")
	if exists(ini) {
		return nil
	}
	src, err := os.ReadFile(filepath.Join(dir, "php.ini-development"))
	if err != nil {
		return err
	}
	enable := map[string]bool{}
	for _, e := range defaultExtensions {
		enable[e] = true
	}
	settings := map[string]string{
		"memory_limit":        "512M",
		"upload_max_filesize": "256M",
		"post_max_size":       "256M",
		"max_execution_time":  "300",
		"max_input_vars":      "5000",
		"date.timezone":       "Europe/Istanbul",
		"extension_dir":       `"ext"`,
	}
	if caFile != "" {
		settings["curl.cainfo"] = `"` + caFile + `"`
		settings["openssl.cafile"] = `"` + caFile + `"`
	}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	enabled := map[string]bool{}
	for i, line := range lines {
		// The real list uses ";extension=x"; doc examples use "; extension=x" and must stay comments.
		if m := iniExtRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(line, ";extension") && enable[m[2]] && !enabled[m[2]] &&
			exists(filepath.Join(dir, "ext", "php_"+m[2]+".dll")) {
			lines[i] = "extension=" + m[2]
			enabled[m[2]] = true
			continue
		}
		trimmed := strings.TrimLeft(strings.TrimSpace(line), "; ")
		for key, val := range settings {
			if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
				if !seen[key] {
					lines[i] = key + " = " + val
					seen[key] = true
				}
			}
		}
	}
	for key, val := range settings {
		if !seen[key] {
			lines = append(lines, key+" = "+val)
		}
	}
	return os.WriteFile(ini, []byte(strings.Join(lines, "\r\n")), 0o644)
}

type PHPExtension struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func listExtensions(dir string) ([]PHPExtension, error) {
	files, err := filepath.Glob(filepath.Join(dir, "ext", "php_*.dll"))
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(dir, "php.ini")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if m := iniExtRe.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil && m[1] == "" {
				enabled[m[2]] = true
			}
		}
	}
	out := []PHPExtension{}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "php_"), ".dll")
		out = append(out, PHPExtension{Name: name, Enabled: enabled[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func setExtension(dir, name string, on bool) error {
	ini := filepath.Join(dir, "php.ini")
	b, err := os.ReadFile(ini)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	directive := "extension=" + name
	if name == "xdebug" || name == "opcache" {
		directive = "zend_extension=" + name
	}
	found := false
	zendRe := regexp.MustCompile(`^\s*;?\s*zend_extension\s*=\s*"?(?:php_)?` + regexp.QuoteMeta(name) + `(?:\.dll)?"?\s*$`)
	for i, line := range lines {
		m := iniExtRe.FindStringSubmatch(line)
		match := (m != nil && m[2] == name) || zendRe.MatchString(line)
		if !match {
			continue
		}
		if on && !found {
			lines[i] = directive
			found = true
		} else {
			lines[i] = ";" + directive
		}
	}
	if on && !found {
		lines = append(lines, directive)
	}
	return os.WriteFile(ini, []byte(strings.Join(lines, "\r\n")), 0o644)
}
