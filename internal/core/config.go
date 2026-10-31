package core

import (
	"encoding/json"
	"os"
)

type SiteConfig struct {
	PHP string `json:"php,omitempty"` // PHP branch, e.g. "8.3"; empty = default
}

type NodeAppConfig struct {
	Node string `json:"node,omitempty"` // Node major, e.g. "24"; empty = default
}

type Config struct {
	WebServer   string                    `json:"webServer"` // "nginx" | "apache"
	HTTPPort    int                       `json:"httpPort"`
	SSL         bool                      `json:"ssl"`
	HTTPSPort   int                       `json:"httpsPort"`
	DBPort      int                       `json:"dbPort"`
	DBPassword  string                    `json:"dbPassword"` // MariaDB root password (local dev only)
	TLD         string                    `json:"tld"`
	DefaultPHP  string                    `json:"defaultPhp"`
	PHPWorkers  int                       `json:"phpWorkers"`
	AutoStart   bool                      `json:"autoStart"`
	CloseToTray bool                      `json:"closeToTray"`
	Sites       map[string]*SiteConfig    `json:"sites"`
	DefaultNode string                    `json:"defaultNode"`
	NodeApps    map[string]*NodeAppConfig `json:"nodeApps"`
}

func defaultConfig() Config {
	return Config{
		WebServer:   "nginx",
		HTTPPort:    80,
		SSL:         true,
		HTTPSPort:   443,
		DBPort:      3306,
		TLD:         "test",
		PHPWorkers:  4,
		CloseToTray: true,
		Sites:       map[string]*SiteConfig{},
	}
}

func loadConfig(path string) Config {
	cfg := defaultConfig()
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	cfg.normalize()
	return cfg
}

func (c *Config) normalize() {
	if c.WebServer != "apache" {
		c.WebServer = "nginx"
	}
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		c.HTTPPort = 80
	}
	if c.HTTPSPort <= 0 || c.HTTPSPort > 65535 || c.HTTPSPort == c.HTTPPort {
		c.HTTPSPort = 443
	}
	if c.DBPort <= 0 || c.DBPort > 65535 {
		c.DBPort = 3306
	}
	if c.TLD == "" {
		c.TLD = "test"
	}
	if c.PHPWorkers < 1 {
		c.PHPWorkers = 1
	}
	if c.PHPWorkers > 9 {
		c.PHPWorkers = 9
	}
	if c.NodeApps == nil {
		c.NodeApps = map[string]*NodeAppConfig{}
	}
	if c.Sites == nil {
		c.Sites = map[string]*SiteConfig{}
	}
}

func saveConfig(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
