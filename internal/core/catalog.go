package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Release describes a downloadable version of a component.
type Release struct {
	Component string `json:"component"` // php, nginx, apache, mariadb, phpmyadmin, composer
	Branch    string `json:"branch"`    // "8.3" for PHP; same as version otherwise
	Version   string `json:"version"`
	URL       string `json:"url"`
	Fallback  string `json:"-"`
}

type Catalog struct {
	PHP        []Release `json:"php"`
	Nginx      *Release  `json:"nginx"`
	Apache     *Release  `json:"apache"`
	MariaDB    *Release  `json:"mariadb"`
	PhpMyAdmin *Release  `json:"phpmyadmin"`
	Composer   *Release  `json:"composer"`
	Errors     []string  `json:"errors"`
}

type catalogCache struct {
	mu      sync.Mutex
	value   *Catalog
	fetched time.Time
}

func (c *catalogCache) get(force bool) *Catalog {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.value != nil && time.Since(c.fetched) < 15*time.Minute {
		return c.value
	}
	c.value = fetchCatalog()
	c.fetched = time.Now()
	return c.value
}

func fetchCatalog() *Catalog {
	cat := &Catalog{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	run := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				cat.Errors = append(cat.Errors, name+": "+err.Error())
				mu.Unlock()
			}
		}()
	}
	run("PHP", func() (err error) { cat.PHP, err = fetchPHPReleases(); return })
	run("Nginx", func() (err error) { cat.Nginx, err = fetchNginx(); return })
	run("Apache", func() (err error) { cat.Apache, err = fetchApache(); return })
	run("MariaDB", func() (err error) { cat.MariaDB, err = fetchMariaDB(); return })
	run("phpMyAdmin", func() (err error) { cat.PhpMyAdmin, err = fetchPhpMyAdmin(); return })
	run("Composer", func() (err error) { cat.Composer, err = fetchComposer(); return })
	wg.Wait()
	return cat
}

const phpDownloads = "https://downloads.php.net/~windows/releases/"

func fetchPHPReleases() ([]Release, error) {
	text, err := fetchText(phpDownloads + "releases.json")
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, err
	}
	var out []Release
	for branch, entry := range raw {
		var version string
		_ = json.Unmarshal(entry["version"], &version)
		for key, val := range entry {
			if !strings.HasPrefix(key, "nts-") || !strings.HasSuffix(key, "-x64") {
				continue
			}
			var build struct {
				Zip struct {
					Path string `json:"path"`
				} `json:"zip"`
			}
			if json.Unmarshal(val, &build) != nil || build.Zip.Path == "" {
				continue
			}
			out = append(out, Release{
				Component: "php",
				Branch:    branch,
				Version:   version,
				URL:       phpDownloads + build.Zip.Path,
				Fallback:  phpDownloads + "archives/" + build.Zip.Path,
			})
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return compareVersions(out[i].Branch, out[j].Branch) > 0 })
	return out, nil
}

func fetchNginx() (*Release, error) {
	text, err := fetchText("https://nginx.org/en/download.html")
	if err != nil {
		return nil, err
	}
	m := regexp.MustCompile(`/download/nginx-([0-9.]+)\.zip`).FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("sürüm bulunamadı")
	}
	return &Release{Component: "nginx", Branch: m[1], Version: m[1], URL: "https://nginx.org" + m[0]}, nil
}

func fetchApache() (*Release, error) {
	text, err := fetchText("https://www.apachelounge.com/download/")
	if err != nil {
		return nil, err
	}
	m := regexp.MustCompile(`/download/[^"']*?/httpd-([0-9.]+)-[0-9]+-[Ww]in64-VS[0-9]+\.zip`).FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("sürüm bulunamadı")
	}
	return &Release{Component: "apache", Branch: m[1], Version: m[1], URL: "https://www.apachelounge.com" + m[0]}, nil
}

func fetchMariaDB() (*Release, error) {
	text, err := fetchText("https://downloads.mariadb.org/rest-api/mariadb/")
	if err != nil {
		return nil, err
	}
	var majors struct {
		Releases []struct {
			ID      string `json:"release_id"`
			Status  string `json:"release_status"`
			Support string `json:"release_support_type"`
		} `json:"major_releases"`
	}
	if err := json.Unmarshal([]byte(text), &majors); err != nil {
		return nil, err
	}
	branch := ""
	for _, r := range majors.Releases {
		if r.Status == "Stable" && strings.Contains(r.Support, "Long Term") {
			branch = r.ID
			break
		}
	}
	if branch == "" {
		return nil, fmt.Errorf("LTS sürümü bulunamadı")
	}
	text, err = fetchText("https://downloads.mariadb.org/rest-api/mariadb/" + branch + "/latest")
	if err != nil {
		return nil, err
	}
	var latest struct {
		Releases map[string]struct {
			Files []struct {
				Name string `json:"file_name"`
				URL  string `json:"file_download_url"`
			} `json:"files"`
		} `json:"releases"`
	}
	if err := json.Unmarshal([]byte(text), &latest); err != nil {
		return nil, err
	}
	for version, rel := range latest.Releases {
		for _, f := range rel.Files {
			if strings.HasSuffix(f.Name, "-winx64.zip") {
				url := strings.Replace(f.URL, "http://", "https://", 1)
				return &Release{Component: "mariadb", Branch: branch, Version: version, URL: url}, nil
			}
		}
	}
	return nil, fmt.Errorf("Windows paketi bulunamadı")
}

func fetchPhpMyAdmin() (*Release, error) {
	text, err := fetchText("https://www.phpmyadmin.net/home_page/version.json")
	if err != nil {
		return nil, err
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(text), &v); err != nil || v.Version == "" {
		return nil, fmt.Errorf("sürüm okunamadı")
	}
	url := fmt.Sprintf("https://files.phpmyadmin.net/phpMyAdmin/%s/phpMyAdmin-%s-all-languages.zip", v.Version, v.Version)
	return &Release{Component: "phpmyadmin", Branch: v.Version, Version: v.Version, URL: url}, nil
}

func fetchComposer() (*Release, error) {
	text, err := fetchText("https://getcomposer.org/versions")
	if err != nil {
		return nil, err
	}
	var v struct {
		Stable []struct {
			Version string `json:"version"`
		} `json:"stable"`
	}
	if err := json.Unmarshal([]byte(text), &v); err != nil || len(v.Stable) == 0 {
		return nil, fmt.Errorf("sürüm okunamadı")
	}
	ver := v.Stable[0].Version
	return &Release{Component: "composer", Branch: ver, Version: ver, URL: "https://getcomposer.org/download/" + ver + "/composer.phar"}, nil
}

// compareVersions compares dotted versions numerically ("8.10" > "8.9").
func compareVersions(a, b string) int {
	pa := strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '-' })
	pb := strings.FieldsFunc(b, func(r rune) bool { return r == '.' || r == '-' })
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}
