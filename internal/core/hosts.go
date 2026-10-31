package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	hostsBegin = "# >>> PAMPP"
	hostsEnd   = "# <<< PAMPP"
)

func hostsPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "drivers", "etc", "hosts")
}

// renderHosts returns the hosts file content with our managed block replaced.
func renderHosts(current string, domains []string) string {
	current = strings.ReplaceAll(current, "\r\n", "\n")
	var kept []string
	inside := false
	for _, line := range strings.Split(current, "\n") {
		// The "myampp" markers come from builds before the rename.
		switch strings.TrimSpace(line) {
		case hostsBegin, "# >>> myampp":
			inside = true
			continue
		case hostsEnd, "# <<< myampp":
			inside = false
			continue
		}
		if !inside {
			kept = append(kept, line)
		}
	}
	out := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if len(domains) > 0 {
		var b strings.Builder
		b.WriteString("\n\n" + hostsBegin + "\n")
		for _, d := range domains {
			fmt.Fprintf(&b, "127.0.0.1 %s\n", d)
		}
		b.WriteString(hostsEnd)
		out += b.String()
	}
	return strings.ReplaceAll(out+"\n", "\n", "\r\n")
}

// hostsUpToDate reports whether the hosts file already contains exactly these domains.
func hostsUpToDate(domains []string) bool {
	b, err := os.ReadFile(hostsPath())
	if err != nil {
		return false
	}
	return renderHosts(string(b), domains) == strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n", "\r\n")
}

// SyncHostsFromFile is run by the elevated helper process (pampp.exe --sync-hosts <file>).
func SyncHostsFromFile(listFile string) error {
	list, err := os.ReadFile(listFile)
	if err != nil {
		return err
	}
	var domains []string
	for _, d := range strings.Fields(string(list)) {
		if isValidHostname(d) {
			domains = append(domains, d)
		}
	}
	current, err := os.ReadFile(hostsPath())
	if err != nil {
		return err
	}
	return os.WriteFile(hostsPath(), []byte(renderHosts(string(current), domains)), 0o644)
}

func isValidHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}
