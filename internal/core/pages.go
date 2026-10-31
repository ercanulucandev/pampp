package core

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed templates/start.html
var startTemplate string

//go:embed templates/site.php
var siteTemplate string

// logoSVG is the PAMPP mark (same artwork as build/appicon.svg, without effects).
const logoSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="48 48 928 928"><defs><linearGradient id="lg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#6366f1"/><stop offset=".55" stop-color="#8b5cf6"/><stop offset="1" stop-color="#c026d3"/></linearGradient><linearGradient id="lb" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fde68a"/><stop offset="1" stop-color="#f59e0b"/></linearGradient></defs><rect x="48" y="48" width="928" height="928" rx="232" fill="url(#lg)"/><path d="M336 776V264H552A176 176 0 0 1 552 616H336" fill="none" stroke="#fff" stroke-width="120" stroke-linecap="round" stroke-linejoin="round"/><path d="M562 352L478 458H530L506 530L592 418H540Z" fill="url(#lb)" stroke="url(#lb)" stroke-width="12" stroke-linejoin="round"/></svg>`

var startTmpl = template.Must(template.New("start").Parse(startTemplate))

type startSite struct{ Domain, URL, PHP string }

// writeStartPage renders the page served at http(s)://localhost.
func (m *Manager) writeStartPage(cfg Config, wc webConfig) error {
	dir := m.P.Data("start")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	installed, _ := m.installedPHP()
	webName := map[string]string{"nginx": "Nginx", "apache": "Apache"}[cfg.WebServer]
	if v := readVersionFile(m.dir(cfg.WebServer)); v != "" {
		webName += " " + v
	}
	m.mu.Lock()
	trusted := m.trusted
	m.mu.Unlock()

	data := map[string]any{
		"Logo":      template.HTML(logoSVG),
		"LogoURI":   template.URL(url.PathEscape(logoSVG)),
		"WebServer": webName,
		"PHP":       installed,
		"MariaDB":   readVersionFile(m.dir("mariadb")),
		"SSL":       cfg.SSL,
		"Trusted":   trusted,
		"TLD":       cfg.TLD,
		"DBPort":    cfg.DBPort,
		"DBPassSet": cfg.DBPassword != "",
		"Root":      m.P.Root,
		"Version":   Version,
	}
	var sites []startSite
	for _, s := range m.sites(cfg) {
		if s.System {
			data["PMA"] = s.URL
			continue
		}
		sites = append(sites, startSite{s.Domain, s.URL, s.PHP})
	}
	data["Sites"] = sites

	var buf bytes.Buffer
	if err := startTmpl.Execute(&buf, data); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "phpinfo.php"), []byte("<?php phpinfo();\n"), 0o644)
}

// siteWelcomePage is the index.php placed in a newly created site.
func siteWelcomePage(name string, dbPort int, dbPassword string) string {
	return strings.NewReplacer("{{SITE}}", name, "{{DBPORT}}", strconv.Itoa(dbPort), "{{DBPASS}}", phpString(dbPassword)).Replace(siteTemplate)
}

// phpString escapes s for use inside a single-quoted PHP string literal.
func phpString(s string) string {
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s)
}
