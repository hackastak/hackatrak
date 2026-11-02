package scan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotDirectory is returned when a scan root exists but is not a directory.
var ErrNotDirectory = errors.New("scan: path is not a directory")

// pruneDirNames are never entered. Nested git repos inside them are ignored,
// and a directory with one of these names is not itself reported as a repo.
var pruneDirNames = map[string]struct{}{
	"vendor":       {},
	"node_modules": {},
}

// DiscoverRepos returns git working trees under root.
//
// A directory is a repo when it contains a .git file or directory (worktrees
// and submodules use a file). The walk does not descend into a repo, so nested
// repos are not returned. It also does not enter vendor or node_modules, and
// it does not follow symlinks except for the scan root itself.
//
// Paths are absolute, symlink-resolved, and sorted. An empty tree returns an empty slice.
func DiscoverRepos(root string) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("discover git repos: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("discover git repos: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("discover git repos: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("discover git repos: %w: %s", ErrNotDirectory, resolved)
	}

	var repos []string
	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path != resolved {
			if _, skip := pruneDirNames[d.Name()]; skip || d.Name() == ".git" {
				return filepath.SkipDir
			}
		}
		repo, err := hasGitMetadata(path)
		if err != nil {
			return err
		}
		if !repo {
			return nil
		}
		repos = append(repos, path)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("discover git repos: %w", err)
	}
	sort.Strings(repos)
	return repos, nil
}

// CurrentBranch returns the branch checked out in repo.
// It runs `git rev-parse --abbrev-ref HEAD`. A detached HEAD is "HEAD",
// which is what that command prints. An unborn branch has no revision yet,
// so rev-parse fails; that case falls back to `git symbolic-ref --short HEAD`.
func CurrentBranch(repo string) (string, error) {
	branch, err := gitOutput(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		return branch, nil
	}
	if alt, altErr := gitOutput(repo, "symbolic-ref", "--short", "HEAD"); altErr == nil {
		return alt, nil
	}
	return "", err
}

func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("current branch of %s: %w", repo, err)
		}
		return "", fmt.Errorf("current branch of %s: %w: %s", repo, err, text)
	}
	if text == "" {
		return "", fmt.Errorf("current branch of %s: empty git output", repo)
	}
	return text, nil
}

// hasGitMetadata reports whether dir has a .git entry. A symlink is enough;
// the target is not required to exist.
func hasGitMetadata(dir string) (bool, error) {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
