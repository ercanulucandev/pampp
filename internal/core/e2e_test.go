package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEndToEnd installs a real stack into PAMPP_E2E_ROOT and serves a site through
// nginx and Apache. It downloads ~250 MB, so it only runs when that variable is set.
func TestEndToEnd(t *testing.T) {
	root := os.Getenv("PAMPP_E2E_ROOT")
	if root == "" {
		t.Skip("set PAMPP_E2E_ROOT to run")
	}
	InitJobObject()
	m := NewManager(root, nil)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.updateConfig(func(c *Config) {
		c.HTTPPort, c.SSL, c.HTTPSPort = 8089, true, 8449
		c.DBPort = 3399
		c.TLD = "localhost"
		c.PHPWorkers = 2
	}))

	cat := m.Catalog(false)
	if len(cat.Errors) > 0 {
		t.Logf("catalog errors: %v", cat.Errors)
	}
	branches := []string{}
	for _, r := range cat.PHP {
		if r.Branch == "8.4" || r.Branch == "8.2" {
			branches = append(branches, r.Branch)
		}
	}
	for _, c := range [][2]string{{"nginx", ""}, {"apache", ""}, {"mariadb", ""}, {"phpmyadmin", ""}, {"composer", ""}, {"php", branches[0]}, {"php", branches[1]}} {
		if c[0] == "php" && exists(m.P.Bin("php", c[1], "php-cgi.exe")) || c[0] != "php" && m.componentInstalled(c[0]) {
			continue
		}
		task := &Task{}
		start := time.Now()
		must(m.install(task, c[0], c[1]))
		t.Logf("installed %s %s in %s", c[0], c[1], time.Since(start).Round(time.Second))
	}

	must(os.MkdirAll(m.P.WWW("demo"), 0o755))
	must(os.WriteFile(m.P.WWW("demo", "index.php"), []byte(`<?php echo "PHP=" . PHP_MAJOR_VERSION . "." . PHP_MINOR_VERSION . " db=" . (new mysqli("127.0.0.1","root","","",3399))->server_info;`), 0o644))
	defer m.Shutdown()

	// No keep-alive: a reused connection would stay on an nginx worker from before the reload.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 30 * time.Second}
	get := func(host, path string) string {
		t.Helper()
		req, _ := http.NewRequest("GET", "http://127.0.0.1:8089"+path, nil)
		req.Host = host
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s%s: %v", host, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	// HTTPS client that trusts only PAMPP's CA (the Windows store is left untouched).
	getTLS := func(host, path string) string {
		t.Helper()
		pool := x509.NewCertPool()
		caPEM, err := os.ReadFile(m.P.Data("ssl", "ca.crt"))
		must(err)
		pool.AppendCertsFromPEM(caPEM)
		tr := &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{RootCAs: pool},
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:8449")
			},
		}
		resp, err := (&http.Client{Transport: tr, Timeout: 30 * time.Second}).Get("https://" + host + ":8449" + path)
		if err != nil {
			t.Fatalf("GET https://%s%s: %v", host, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	for _, server := range []string{"nginx", "apache"} {
		m.StopService("web")
		must(m.updateConfig(func(c *Config) { c.WebServer = server; c.Sites = map[string]*SiteConfig{} }))
		must(m.StartAll())

		body := get("demo.localhost", "/")
		t.Logf("[%s] demo: %s", server, body)
		if !strings.HasPrefix(body, "PHP="+branches[0]) || !strings.Contains(body, "MariaDB") {
			t.Fatalf("[%s] unexpected body: %s", server, body)
		}

		must(m.SetSitePHP("demo", branches[1]))
		for i := 0; i < 10; i++ {
			if body = get("demo.localhost", "/"); strings.Contains(body, "PHP="+branches[1]) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Logf("[%s] after switch: %s", server, body)
		if !strings.Contains(body, "PHP="+branches[1]) {
			conf, _ := os.ReadFile(m.P.Etc(server, map[string]string{"nginx": "nginx.conf", "apache": "httpd.conf"}[server]))
			t.Fatalf("[%s] PHP switch failed: %s\n--- config ---\n%s", server, body, conf)
		}

		pma := get("phpmyadmin.localhost", "/")
		if !strings.Contains(pma, "phpMyAdmin") {
			t.Fatalf("[%s] phpMyAdmin not served: %.300s", server, pma)
		}
		t.Logf("[%s] phpMyAdmin OK (%d bytes)", server, len(pma))

		if body := getTLS("demo.localhost", "/"); !strings.HasPrefix(body, "PHP=") {
			t.Fatalf("[%s] HTTPS failed: %.300s", server, body)
		}
		if start := getTLS("localhost", "/"); !strings.Contains(start, "Her şey hazır") || !strings.Contains(start, "demo.localhost") {
			t.Fatalf("[%s] start page missing: %.300s", server, start)
		}
		t.Logf("[%s] HTTPS + start page OK", server)
	}

	site, err := m.CreateSite("welcome", branches[0])
	must(err)
	page := getTLS(site.Domain, "/")
	if !strings.Contains(page, "welcome.localhost hazır!") || !strings.Contains(page, "Bağlı") || !strings.Contains(page, "HTTPS") {
		t.Fatalf("welcome page wrong: %.500s", page)
	}
	t.Logf("new site %s served its welcome page over HTTPS with DB connected", site.URL)

	exts, err := m.Extensions(branches[0])
	must(err)
	on := 0
	for _, e := range exts {
		if e.Enabled {
			on++
		}
	}
	t.Logf("PHP %s: %d/%d extensions enabled; ini at %s", branches[0], on, len(exts), filepath.Join(m.P.Bin("php", branches[0]), "php.ini"))
}
