package scan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hackastak/hackatrak/internal/config"
)

// ErrNoGitProject is returned when no ancestor of a path contains .git.
var ErrNoGitProject = errors.New("scan: no git project")

// ErrNoProject is returned when a path has no git ancestor and no first
// path segment under a matching root.
var ErrNoProject = errors.New("scan: no project")

// ErrNoBucket is returned when no roots[].path prefixes the path.
var ErrNoBucket = errors.New("scan: no bucket")

// Project is the directory treated as one unit of work.
// Root is the git working tree, or the first path segment under the matching scan root.
// Name is that directory's basename. Slug is the Obsidian wikilink target, the repo dir name.
type Project struct {
	Root string
	Name string
	Slug string
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
	return newProject(filepath.Join(match, seg))
}

// WikilinkSlug returns the Obsidian wikilink target for a project directory.
// It is the repo directory name, unchanged: hyphens, dots, spaces, and case stay,
// so [[supply-chain-monitor]] matches the folder. It does not walk to a git root;
// pass the project directory from ResolveProject.
func WikilinkSlug(projectRoot string) (string, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return "", fmt.Errorf("wikilink slug: empty path")
	}
	cleaned := filepath.Clean(root)
	if cleaned == "." || cleaned == ".." || isFilesystemRoot(cleaned) {
		return "", fmt.Errorf("wikilink slug: %s is not a project directory", projectRoot)
	}
	slug := filepath.Base(cleaned)
	if slug == "" || slug == "." || slug == ".." || isFilesystemRoot(slug) {
		return "", fmt.Errorf("wikilink slug: %s is not a project directory", projectRoot)
	}
	return slug, nil
}

func isFilesystemRoot(path string) bool {
	return path == string(filepath.Separator)
}

func newProject(root string) (Project, error) {
	slug, err := WikilinkSlug(root)
	if err != nil {
		return Project{}, err
	}
	return Project{Root: root, Name: slug, Slug: slug}, nil
}

// ResolveBucket returns the bucket of the longest roots[].path that prefixes path.
// Matching is a path boundary, not a raw string prefix, so Developer does not
// match DeveloperExtra. The path may be the root itself. Paths are canonicalized
// the same way as project resolution, so a symlinked root still matches.
// An empty path is an error that does not wrap ErrNoBucket. A path under no
// root returns ErrNoBucket. A matching root with an empty bucket is an error.
func ResolveBucket(path string, roots []config.Root) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("resolve bucket: empty path")
	}
	abs, err := canonicalPath(path)
	if err != nil {
		return "", fmt.Errorf("resolve bucket: %w", err)
	}
	match, bucket, ok := longestMatchingBucket(abs, roots)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoBucket, path)
	}
	if strings.TrimSpace(bucket) == "" {
		return "", fmt.Errorf("resolve bucket: empty bucket for %s", match)
	}
	return bucket, nil
}

func longestMatchingBucket(path string, roots []config.Root) (string, string, bool) {
	var best, bucket string
	for _, root := range roots {
		rootPath := strings.TrimSpace(root.Path)
		if rootPath == "" {
			continue
		}
		canon, err := canonicalPath(rootPath)
		if err != nil {
			continue
		}
		if !pathWithin(canon, path) {
			continue
		}
		if len(canon) > len(best) {
			best = canon
			bucket = root.Bucket
		}
	}
	return best, bucket, best != ""
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
			return newProject(dir)
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
