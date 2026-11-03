package scan

import (
	"bytes"
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

// ErrNotRepository is returned when a path is not itself a git working tree.
// A directory inside a repo is not a repo; git's parent walk is not used.
var ErrNotRepository = errors.New("scan: not a git repository")

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
// which is what that command prints. An empty repo has no revision yet,
// so rev-parse fails; that case falls back to `git symbolic-ref --short HEAD`.
// A directory that is not itself a repo returns ErrNotRepository.
func CurrentBranch(repo string) (string, error) {
	if err := requireRepo(repo); err != nil {
		return "", err
	}
	branch, err := gitOutput(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		return branch, nil
	}
	if alt, altErr := gitOutput(repo, "symbolic-ref", "--short", "HEAD"); altErr == nil {
		return alt, nil
	}
	return "", err
}

// detachedHEAD reports whether HEAD is detached.
// An empty repo has no HEAD revision; that is not detached.
func detachedHEAD(repo string) (bool, error) {
	text, err := runGit(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		if emptyHistory(err) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(text) == "HEAD", nil
}

func emptyHistory(err error) bool {
	return noCommitsYet(err) || strings.Contains(err.Error(), "ambiguous argument 'HEAD'")
}

func noCommitsYet(err error) bool {
	return strings.Contains(err.Error(), "does not have any commits yet")
}

// requireRepo rejects paths that are not themselves git working trees.
// It does not walk parent directories.
func requireRepo(repo string) error {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNotRepository, repo)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotRepository, abs)
		}
		return fmt.Errorf("git repo %s: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotDirectory, abs)
	}
	ok, err := hasGitMetadata(abs)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotRepository, abs)
	}
	return nil
}

func gitOutput(repo string, args ...string) (string, error) {
	text, err := runGit(repo, args...)
	if err != nil {
		return "", fmt.Errorf("current branch of %s: %w", repo, err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("current branch of %s: empty git output", repo)
	}
	return text, nil
}

// runGit runs git -C repo and returns stdout. stderr is attached to the error.
func runGit(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo, "--no-pager"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return "", err
		}
		if strings.Contains(msg, "not a git repository") {
			return "", fmt.Errorf("%w: %s", ErrNotRepository, msg)
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}
	return stdout.String(), nil
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
