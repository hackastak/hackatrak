package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEmptyRepoHasNoCommits(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")

	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch() = %q, want main", branch)
	}

	got, err := CommitsSince(repo, "2026-11-02T00:00:00-06:00", []string{"me@example.com"})
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("CommitsSince() = %#v, want none", got)
	}
}

func TestCommitsSinceDetachedHEAD(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	gitCommit(t, repo, "2026-11-02T09:00:00-06:00", "me@example.com", "Me", "on main", "")
	git(t, repo, "checkout", "--detach")
	gitCommit(t, repo, "2026-11-02T10:00:00-06:00", "me@example.com", "Me", "detached only", "body line")

	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "HEAD" {
		t.Fatalf("CurrentBranch() = %q, want HEAD", branch)
	}

	got, err := CommitsSince(repo, "2026-11-02T00:00:00-06:00", []string{"me@example.com"})
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("CommitsSince() len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Subject != "on main" || got[0].Branch != "main" {
		t.Fatalf("branch commit = %#v, want main/on main", got[0])
	}
	if got[1].Subject != "detached only" || got[1].Branch != "HEAD" || got[1].Body != "body line" {
		t.Fatalf("detached commit = %#v, want HEAD/detached only", got[1])
	}
}

func TestCommitsSinceNoCommitsToday(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	gitCommit(t, repo, "2026-11-01T12:00:00-06:00", "me@example.com", "Me", "yesterday", "")

	got, err := CommitsSince(repo, "2026-11-02T00:00:00-06:00", []string{"me@example.com"})
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("CommitsSince() = %#v, want none", got)
	}
}

func TestNonRepoDirIsNotAParentRepo(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	gitCommit(t, repo, "2026-11-02T09:00:00-06:00", "me@example.com", "Me", "in parent", "")

	notes := filepath.Join(repo, "notes")
	if err := os.MkdirAll(filepath.Join(notes, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notes, "today.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := CurrentBranch(notes); !errors.Is(err, ErrNotRepository) {
		t.Fatalf("CurrentBranch(non-repo) error = %v, want ErrNotRepository", err)
	}
	if _, err := CommitsSince(notes, "2026-11-02T00:00:00-06:00", []string{"me@example.com"}); !errors.Is(err, ErrNotRepository) {
		t.Fatalf("CommitsSince(non-repo) error = %v, want ErrNotRepository", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "loose", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "loose", "docs", "note.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverRepos(root)
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DiscoverRepos() = %#v, want none", got)
	}
}
