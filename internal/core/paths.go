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

// ResolveRoot returns where PAMPP keeps its data: the folder next to the executable
// (C:\pampp for the installer, any folder for the portable build). Only a copy placed
// under Program Files, which normal users cannot write to, falls back to %USERPROFILE%\PAMPP.
// PAMPP_HOME overrides everything, which is handy during development.
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
	dir := filepath.Dir(exe)
	if DataInProfile(dir) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "PAMPP")
		}
	}
	return dir
}

// IsInstalled reports whether exeDir holds an installer-made copy (the NSIS uninstaller sits next to it).
func IsInstalled(exeDir string) bool {
	return exists(filepath.Join(exeDir, "uninstall.exe"))
}

// DataInProfile reports whether data must live in %USERPROFILE%\PAMPP because exeDir is
// under Program Files. A pampp.portable marker or existing pampp.json keeps data local.
func DataInProfile(exeDir string) bool {
	if exists(filepath.Join(exeDir, "pampp.portable")) || exists(filepath.Join(exeDir, "pampp.json")) {
		return false
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		if pf := os.Getenv(env); pf != "" && strings.HasPrefix(strings.ToLower(exeDir), strings.ToLower(pf)+`\`) {
			return true
		}
	}
	return false
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
func (p Paths) Node(parts ...string) string { return p.join(append([]string{"nodejs"}, parts...)...) }
func (p Paths) ConfigFile() string          { return p.join("pampp.json") }

// Ensure creates the base directory layout.
func (p Paths) Ensure() error {
	for _, d := range []string{p.Bin(), p.Etc("nginx", "sites"), p.Etc("apache", "sites"), p.Etc("mariadb"), p.Data(), p.Logs(), p.Tmp("downloads"), p.WWW(), p.Node()} {
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
