package core

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) PAMPP/1.0"

var httpClient = &http.Client{Timeout: 30 * time.Minute}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return resp, nil
}

func fetchText(url string) (string, error) {
	resp, err := httpGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	return string(b), err
}

type progressFunc func(done, total int64)

type progressWriter struct {
	done, total int64
	fn          progressFunc
	last        time.Time
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	if w.fn != nil && time.Since(w.last) > 150*time.Millisecond {
		w.last = time.Now()
		w.fn(w.done, w.total)
	}
	return len(p), nil
}

// download saves url to dest, reporting progress along the way.
func download(url, dest string, fn progressFunc) error {
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	pw := &progressWriter{total: resp.ContentLength, fn: fn}
	_, err = io.Copy(io.MultiWriter(f, pw), resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if fn != nil {
		fn(pw.done, pw.done)
	}
	os.Remove(dest)
	return os.Rename(tmp, dest)
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	base := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, base) {
			return fmt.Errorf("arşivde geçersiz yol: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// findMarker returns the directory (root or up to two levels below) that contains marker.
func findMarker(root, marker string) string {
	if exists(filepath.Join(root, marker)) {
		return root
	}
	var found string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.Count(rel, string(os.PathSeparator)) > 1 {
			return filepath.SkipDir
		}
		if exists(filepath.Join(p, marker)) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// installArchive extracts zipPath and atomically swaps it into finalDir.
// Files listed in keep are carried over from the previous installation.
func installArchive(zipPath, finalDir, marker string, keep []string) error {
	staging := finalDir + ".new"
	old := finalDir + ".old"
	os.RemoveAll(staging)
	os.RemoveAll(old)
	if err := extractZip(zipPath, staging); err != nil {
		os.RemoveAll(staging)
		return err
	}
	root := findMarker(staging, marker)
	if root == "" {
		os.RemoveAll(staging)
		return fmt.Errorf("arşiv beklenen dosyayı içermiyor: %s", marker)
	}
	if exists(finalDir) {
		if err := os.Rename(finalDir, old); err != nil {
			os.RemoveAll(staging)
			return fmt.Errorf("eski sürüm taşınamadı (çalışıyor olabilir): %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(root, finalDir); err != nil {
		if exists(old) {
			os.Rename(old, finalDir)
		}
		os.RemoveAll(staging)
		return err
	}
	for _, k := range keep {
		if b, err := os.ReadFile(filepath.Join(old, k)); err == nil {
			_ = os.WriteFile(filepath.Join(finalDir, k), b, 0o644)
		}
	}
	os.RemoveAll(staging)
	os.RemoveAll(old)
	return nil
}
