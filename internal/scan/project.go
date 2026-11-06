package scan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoGitProject is returned when no ancestor of a path contains .git.
var ErrNoGitProject = errors.New("scan: no git project")

// ErrNoProject is returned when a path has no git ancestor and no first
// path segment under a matching root.
var ErrNoProject = errors.New("scan: no project")

// Project is the directory treated as one unit of work.
// Root is the git working tree, or the first path segment under the matching scan root.
// Name is that directory's basename.
type Project struct {
	Root string
	Name string
}

// GitProject is the git-only view of Project.
type GitProject = Project

// ResolveProject returns the git project for path when one exists.
// Otherwise it returns the first path segment under the longest matching root.
// roots are scan-root paths. A path that is itself a root, or that sits under
// none of them and has no git ancestor, returns ErrNoProject.
func ResolveProject(path string, roots []string) (Project, error) {
	git, err := ResolveGitProject(path)
	if err == nil {
		return git, nil
	}
	if !errors.Is(err, ErrNoGitProject) {
		return Project{}, err
	}
	return fallbackProject(path, roots)
}

func fallbackProject(path string, roots []string) (Project, error) {
	abs, err := canonicalPath(path)
	if err != nil {
		return Project{}, err
	}
	match, ok := longestMatchingRoot(abs, roots)
	if !ok {
		return Project{}, fmt.Errorf("%w: %s", ErrNoProject, path)
	}
	rel, err := filepath.Rel(match, abs)
	if err != nil || rel == "." || rel == "" {
		return Project{}, fmt.Errorf("%w: %s", ErrNoProject, path)
	}
	seg := rel
	if i := strings.Index(rel, string(filepath.Separator)); i >= 0 {
		seg = rel[:i]
	}
	if seg == "" || seg == "." || seg == ".." {
		return Project{}, fmt.Errorf("%w: %s", ErrNoProject, path)
	}
	return Project{Root: filepath.Join(match, seg), Name: seg}, nil
}

func longestMatchingRoot(path string, roots []string) (string, bool) {
	var best string
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		canon, err := canonicalPath(root)
		if err != nil {
			continue
		}
		if !pathWithin(canon, path) {
			continue
		}
		if len(canon) > len(best) {
			best = canon
		}
	}
	return best, best != ""
}

func pathWithin(root, path string) bool {
	if root == path {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func canonicalPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("resolve project: empty path")
	}
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("resolve project: %w", err)
	}
	return resolveExistingPrefix(abs), nil
}

// ResolveGitProject returns the nearest ancestor of path that contains .git,
// including path itself when that directory is the working tree.
// A file path starts the walk at its parent. A .git file counts the same as
// a .git directory. Paths with no such ancestor return ErrNoGitProject.
func ResolveGitProject(path string) (GitProject, error) {
	dir, err := gitProjectWalkStart(path)
	if err != nil {
		return GitProject{}, err
	}
	for {
		ok, err := hasGitMetadata(dir)
		if err != nil {
			return GitProject{}, fmt.Errorf("resolve git project: %w", err)
		}
		if ok {
			return GitProject{Root: dir, Name: filepath.Base(dir)}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return GitProject{}, fmt.Errorf("%w: %s", ErrNoGitProject, path)
		}
		dir = parent
	}
}

func gitProjectWalkStart(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("resolve git project: empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve git project: %w", err)
	}
	abs = resolveExistingPrefix(abs)
	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return abs, nil
}

// resolveExistingPrefix symlink-resolves the longest existing ancestor and
// rejoins any missing suffix. A path that does not exist yet still needs a
// stable root (on macOS /var is /private/var).
func resolveExistingPrefix(path string) string {
	path = filepath.Clean(path)
	var missing []string
	cur := path
	for {
		if _, err := os.Lstat(cur); err == nil {
			resolved, err := filepath.EvalSymlinks(cur)
			if err != nil {
				break
			}
			if len(missing) == 0 {
				return resolved
			}
			parts := append([]string{resolved}, missing...)
			return filepath.Join(parts...)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		missing = append([]string{filepath.Base(cur)}, missing...)
		cur = parent
	}
	return path
}
