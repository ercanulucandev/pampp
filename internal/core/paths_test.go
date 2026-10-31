package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallModes(t *testing.T) {
	touch := func(dir, name string) {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Installer copy in C:\pampp style folder: installed, data stays next to the exe.
	installed := t.TempDir()
	touch(installed, "uninstall.exe")
	if !IsInstalled(installed) || DataInProfile(installed) {
		t.Error("installer folder must be installed and keep data locally")
	}

	portable := t.TempDir()
	if IsInstalled(portable) || DataInProfile(portable) {
		t.Error("plain folder must be portable with local data")
	}

	// Under Program Files nobody can write next to the exe, so data moves to the profile.
	t.Setenv("ProgramFiles", filepath.Dir(portable))
	if !DataInProfile(portable) {
		t.Error("folder under Program Files must keep data in the profile")
	}
	touch(portable, "pampp.portable")
	if DataInProfile(portable) {
		t.Error("pampp.portable marker must force local data")
	}

	if dir := os.Getenv("PAMPP_INSTALL_DIR"); dir != "" && !IsInstalled(dir) {
		t.Errorf("real installer output %s not detected as installed", dir)
	}
}
