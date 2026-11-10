package spool

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hackastak/hackatrak/internal/config"
)

// Write atomically replaces path with file.
// The parent directory is created. Equal content is left untouched, so a
// re-run does not churn Obsidian Sync. The weekly note is not involved.
// wrote is false when the file already had these bytes.
func Write(path string, file File) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, fmt.Errorf("spool: path is empty")
	}
	data, err := file.Bytes()
	if err != nil {
		return false, err
	}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("spool: read %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("spool: create directory: %w", err)
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("spool: stat %s: %w", path, err)
	}
	if err := replace(path, data, mode); err != nil {
		return false, err
	}
	return true, nil
}

// Read parses one spool file and checks the filename against its contents.
// A file named for a different account or week is rejected.
func Read(path string) (File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return File{}, fmt.Errorf("spool: path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("spool: read %s: %w", path, err)
	}
	file, err := Parse(data)
	if err != nil {
		return File{}, err
	}
	want, err := Name(file.Account, file.Year, file.Week)
	if err != nil {
		return File{}, err
	}
	if filepath.Base(path) != want {
		return File{}, fmt.Errorf("spool: %s does not match %s", filepath.Base(path), want)
	}
	return file, nil
}

// Discover returns this ISO week's spool files in dir, sorted by path.
// A missing directory is empty, not an error. Names that are not
// spool-<account>-<YYYY>-W<ww>.json are ignored. The files are not parsed.
func Discover(dir string, year, week int) ([]string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("spool: directory is empty")
	}
	if _, err := Name("hackatrak", year, week); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("spool: read directory: %w", err)
	}
	suffix := fmt.Sprintf("-%04d-W%02d.json", year, week)
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "spool-") || !strings.HasSuffix(name, suffix) {
			continue
		}
		account := strings.TrimSuffix(strings.TrimPrefix(name, "spool-"), suffix)
		if err := config.CheckAccount(account); err != nil {
			continue
		}
		want, err := Name(account, year, week)
		if err != nil || want != name {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	sort.Strings(paths)
	return paths, nil
}

func replace(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".spool-*.tmp")
	if err != nil {
		return fmt.Errorf("spool: create temp: %w", err)
	}
	tmp := f.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return fmt.Errorf("spool: chmod temp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("spool: write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("spool: sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("spool: close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("spool: rename temp: %w", err)
	}
	success = true
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}
