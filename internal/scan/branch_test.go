package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentBranch(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")

	got, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if got != "main" {
		t.Fatalf("CurrentBranch() = %q, want main", got)
	}

	git(t, repo, "checkout", "-b", "feature/p1.2")
	got, err = CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if got != "feature/p1.2" {
		t.Fatalf("CurrentBranch() = %q, want feature/p1.2", got)
	}
}

func TestCurrentBranchDetachedHEAD(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "--allow-empty", "-m", "init")
	git(t, repo, "checkout", "--detach")

	got, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if got != "HEAD" {
		t.Fatalf("CurrentBranch() = %q, want HEAD", got)
	}
}

func TestCurrentBranchRejectsNonRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := CurrentBranch(dir)
	if err == nil {
		t.Fatal("CurrentBranch() error = nil, want non-repo failure")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("CurrentBranch() error = %v, want not a git repository", err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required for branch tests")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gitconfig"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
