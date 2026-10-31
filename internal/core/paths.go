package core

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths resolves every on-disk location relative to the portable root folder.
type Paths struct {
	Root string
}

// ResolveRoot returns the folder next to the executable (portable layout).
// PAMPP_HOME overrides it, which is handy during development.
func ResolveRoot() string {
	if v := os.Getenv("PAMPP_HOME"); v != "" {
		return v
	}
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	exe, _ = filepath.EvalSymlinks(exe)
	return filepath.Dir(exe)
}

func (p Paths) join(parts ...string) string {
	return filepath.Join(append([]string{p.Root}, parts...)...)
}

func (p Paths) Bin(parts ...string) string  { return p.join(append([]string{"bin"}, parts...)...) }
func (p Paths) Etc(parts ...string) string  { return p.join(append([]string{"etc"}, parts...)...) }
func (p Paths) Data(parts ...string) string { return p.join(append([]string{"data"}, parts...)...) }
func (p Paths) Logs(parts ...string) string { return p.join(append([]string{"logs"}, parts...)...) }
func (p Paths) Tmp(parts ...string) string  { return p.join(append([]string{"tmp"}, parts...)...) }
func (p Paths) WWW(parts ...string) string  { return p.join(append([]string{"www"}, parts...)...) }
func (p Paths) ConfigFile() string          { return p.join("pampp.json") }

// Ensure creates the base directory layout.
func (p Paths) Ensure() error {
	for _, d := range []string{p.Bin(), p.Etc("nginx", "sites"), p.Etc("apache", "sites"), p.Etc("mariadb"), p.Data(), p.Logs(), p.Tmp("downloads"), p.WWW()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// fwd converts a Windows path to forward slashes, which both nginx and Apache prefer.
func fwd(p string) string { return strings.ReplaceAll(p, `\`, "/") }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readVersionFile(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeVersionFile(dir, v string) error {
	return os.WriteFile(filepath.Join(dir, ".version"), []byte(v), 0o644)
}
