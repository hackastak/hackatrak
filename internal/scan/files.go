package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// buildDirNames are generated trees. They are skipped by directory name, so a
// source file such as build.go is not excluded. The walk root itself is never
// skipped, even if its basename is in this set.
var buildDirNames = map[string]struct{}{
	".git":         {},
	".next":        {},
	".turbo":       {},
	"__pycache__":  {},
	"build":        {},
	"coverage":     {},
	"dist":         {},
	"node_modules": {},
	"target":       {},
	"vendor":       {},
}

// builtinFileExcludes always apply, in addition to config exclude_globs.
// Patterns are file names, not paths. * does not cross a separator.
var builtinFileExcludes = []string{
	"*.class",
	"*.o",
	"*.pyc",
	"*.swp",
	"*~",
	".DS_Store",
}

// TouchedFile is a regular file whose mtime is inside the scan window.
// Path and Root are absolute and symlink-resolved. Root is the walk root
// the file was found under.
type TouchedFile struct {
	Path  string
	Root  string
	MTime time.Time
}

// FilesSince walks each root for regular files with mtime >= since.
//
// roots are configured scan roots or individual repos. The walk does not
// follow symlinks except for a root that is itself a symlink. A file found
// under more than one root is returned once, attributed to the first root.
// Paths are absolute, symlink-resolved, and sorted by path.
//
// since is the start of the window, normally local midnight. It must be
// non-zero. An empty tree returns an empty slice.
//
// Built-in exclusions always apply: .git, known build-output directories,
// editor temp (*.swp, *~), .DS_Store, and stray compiler artifacts (*.o,
// *.pyc, *.class). excludeGlobs are extra name or relative-path patterns
// from config. A pattern containing / is matched against the slash-separated
// path relative to that walk root; any other pattern is matched against the
// base name. A matching directory is not entered. * does not cross a
// separator, and ** is not special.
//
// A .gitignore in the walk root, or in a directory entered below it, is
// honored. If the walk root sits inside a git repo, .gitignore files from
// that repo root down to the walk root apply too. A .gitignore above a
// non-repo walk root does not. info/exclude and the global excludes file
// are not read. A matching directory is not entered. Last match wins, so
// a later ! pattern can bring a file back unless a parent directory is
// itself ignored.
func FilesSince(roots []string, since time.Time, excludeGlobs ...string) ([]TouchedFile, error) {
	roots, err := collectRoots(roots)
	if err != nil {
		return nil, fmt.Errorf("files since: %w", err)
	}
	if since.IsZero() {
		return nil, fmt.Errorf("files since: time is required")
	}
	globs, err := compileExcludeGlobs(excludeGlobs)
	if err != nil {
		return nil, fmt.Errorf("files since: %w", err)
	}

	var found []TouchedFile
	seen := make(map[string]struct{})
	for _, root := range roots {
		resolved, err := resolveScanRoot(root)
		if err != nil {
			return nil, fmt.Errorf("files since: %w", err)
		}
		ignored, err := newIgnoreState(resolved)
		if err != nil {
			return nil, fmt.Errorf("files since: %w", err)
		}
		err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			ignored.trim(path)
			if d.IsDir() {
				if path == resolved {
					return nil
				}
				skip, err := skipDir(resolved, path, d.Name(), globs)
				if err != nil {
					return err
				}
				if skip || ignored.ignored(path, true) {
					return filepath.SkipDir
				}
				if err := ignored.enter(path); err != nil {
					return err
				}
				return nil
			}
			if !d.Type().IsRegular() || d.Name() == ".git" {
				return nil
			}
			rel, err := filepath.Rel(resolved, path)
			if err != nil {
				return err
			}
			excluded, err := excludedPath(d.Name(), rel, globs)
			if err != nil {
				return err
			}
			if excluded || ignored.ignored(path, false) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.ModTime().Before(since) {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			found = append(found, TouchedFile{
				Path:  path,
				Root:  resolved,
				MTime: info.ModTime(),
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("files since: %w", err)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		return found[i].Path < found[j].Path
	})
	return found, nil
}

func resolveScanRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrNotDirectory, resolved)
	}
	return resolved, nil
}

type excludeGlob struct {
	pattern string
	path    bool
}

func compileExcludeGlobs(extra []string) ([]excludeGlob, error) {
	patterns := make([]string, 0, len(builtinFileExcludes)+len(extra))
	patterns = append(patterns, builtinFileExcludes...)
	patterns = append(patterns, extra...)

	globs := make([]excludeGlob, 0, len(patterns))
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			return nil, fmt.Errorf("exclude glob is empty")
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("exclude glob %q: %w", pattern, err)
		}
		globs = append(globs, excludeGlob{
			pattern: pattern,
			path:    strings.Contains(pattern, "/"),
		})
	}
	return globs, nil
}

func skipDir(root, dir, name string, globs []excludeGlob) (bool, error) {
	if _, skip := buildDirNames[name]; skip {
		return true, nil
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false, err
	}
	return excludedPath(name, rel, globs)
}

func excludedPath(name, rel string, globs []excludeGlob) (bool, error) {
	rel = filepath.ToSlash(rel)
	for _, glob := range globs {
		target := name
		if glob.path {
			target = rel
		}
		ok, err := path.Match(glob.pattern, target)
		if err != nil {
			return false, fmt.Errorf("exclude glob %q: %w", glob.pattern, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
