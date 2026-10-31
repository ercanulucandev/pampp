package core

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNodeEndToEnd installs Node.js into PAMPP_E2E_ROOT, scaffolds projects and runs
// their scripts. It needs network access, so it only runs when that variable is set.
func TestNodeEndToEnd(t *testing.T) {
	root := os.Getenv("PAMPP_E2E_ROOT")
	if root == "" {
		t.Skip("set PAMPP_E2E_ROOT to run")
	}
	InitJobObject()
	m := NewManager(root, nil)
	defer m.Shutdown()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	waitFor := func(what string, timeout time.Duration, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if ok() {
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", what)
	}
	app := func(name string) *NodeApp {
		a, _ := m.appByName(name)
		return a
	}
	script := func(a *NodeApp, name string) NodeScript {
		for _, s := range a.Scripts {
			if s.Name == name {
				return s
			}
		}
		return NodeScript{}
	}
	finished := func(name, key string) func() bool {
		return func() bool {
			m.mu.Lock()
			r := m.nodeRuns[runKey(name, key)]
			m.mu.Unlock()
			return r != nil && !r.running()
		}
	}
	exitOK := func(name, key string) {
		t.Helper()
		m.mu.Lock()
		r := m.nodeRuns[runKey(name, key)]
		m.mu.Unlock()
		if c := r.exitCode(); c == nil || *c != 0 {
			t.Fatalf("%s %s failed:\n%s", name, key, strings.Join(m.NodeLog(name, key), "\n"))
		}
	}

	// 1. Runtime
	if nodes, _ := m.installedNode(); len(nodes) == 0 {
		var lts string
		for _, r := range m.Catalog(false).Node {
			if r.LTS != "" {
				lts = r.Branch
				break
			}
		}
		start := time.Now()
		must(m.installNode(&Task{}, lts))
		t.Logf("installed Node %s in %s", lts, time.Since(start).Round(time.Second))
	}
	st := m.nodeState(m.config())
	t.Logf("default node: %s (%v)", st.Default, st.Installed)

	for _, n := range []string{"api", "web"} {
		os.RemoveAll(m.P.Node(n))
	}

	// 2. Express: create, install, dev server with URL detection
	must(m.CreateNodeApp("api", "express"))
	waitFor("express install", 3*time.Minute, finished("api", installKey))
	exitOK("api", installKey)
	a := app("api")
	if a.Framework != "Express" || !a.HasModules || a.Manager != "npm" || script(a, "dev").Command == "" {
		t.Fatalf("express app detected wrong: %+v", a)
	}
	must(m.RunScript("api", "dev"))
	waitFor("dev URL", 30*time.Second, func() bool { return script(app("api"), "dev").URL != "" })
	url := script(app("api"), "dev").URL
	resp, err := http.Get(url)
	must(err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "api çalışıyor") {
		t.Fatalf("dev server body: %s", body)
	}
	t.Logf("express dev server answered at %s", url)
	m.StopScript("api", "dev")
	if script(app("api"), "dev").Running {
		t.Fatal("dev server still running after stop")
	}
	if _, err := http.Get(url); err == nil {
		t.Fatal("port still answering after stop")
	}

	// 3. Vite + React: scaffold via create-vite, auto install, build
	must(m.CreateNodeApp("web", "vite-react"))
	waitFor("vite scaffold", 3*time.Minute, finished("web", createKey))
	exitOK("web", createKey)
	waitFor("vite install", 5*time.Minute, finished("web", installKey))
	exitOK("web", installKey)
	w := app("web")
	if w.Framework != "Vite" || script(w, "build").Command == "" {
		t.Fatalf("vite app detected wrong: %+v", w)
	}
	must(m.RunScript("web", "build"))
	waitFor("vite build", 3*time.Minute, finished("web", "build"))
	exitOK("web", "build")
	if !exists(filepath.Join(m.P.Node("web"), "dist", "index.html")) {
		t.Fatal("build produced no dist/index.html")
	}
	t.Logf("vite build OK; scripts: %v", func() (n []string) {
		for _, s := range app("web").Scripts {
			n = append(n, s.Name)
		}
		return
	}())

	// 4. Next.js scaffold (large download, opt-in)
	if os.Getenv("PAMPP_E2E_NEXT") == "" {
		return
	}
	os.RemoveAll(m.P.Node("site"))
	must(m.CreateNodeApp("site", "next"))
	waitFor("next scaffold", 8*time.Minute, finished("site", createKey))
	exitOK("site", createKey)
	n := app("site")
	if n == nil || n.Framework != "Next.js" || !n.HasModules || script(n, "dev").Command == "" {
		t.Fatalf("next app detected wrong: %+v", n)
	}
	must(m.RunScript("site", "dev"))
	waitFor("next dev URL", 90*time.Second, func() bool { return script(app("site"), "dev").URL != "" })
	t.Logf("next dev server at %s", script(app("site"), "dev").URL)
	m.StopScript("site", "dev")
}
