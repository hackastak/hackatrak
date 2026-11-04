package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestCollectGitDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")
	now := time.Date(2026, 11, 3, 15, 0, 0, 0, time.FixedZone("CST", -6*3600))
	gitCommit(t, repo, now.Add(-48*time.Hour).Format(time.RFC3339), "me@example.com", "Me", "yesterday", "")
	gitCommit(t, repo, now.Add(-2*time.Minute).Format(time.RFC3339), "me@example.com", "Me", "ship collect", "")
	gitCommit(t, repo, now.Add(-time.Minute).Format(time.RFC3339), "teammate@example.com", "Them", "not mine", "")

	db := openScanDB(t)
	got, err := CollectGit(db, []string{" " + root + " "}, []string{"me@example.com"}, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Commits) != 1 || got.Commits[0].Subject != "ship collect" {
		t.Fatalf("commits = %+v, want only ship collect", got.Commits)
	}
	if got.Inserted != 0 || got.Skipped != 0 {
		t.Fatalf("dry-run wrote counts: %+v", got)
	}
	if got.Commits[0].RepoPath == "" || got.Commits[0].Hash == "" || got.Commits[0].Branch != "main" {
		t.Fatalf("detected = %+v", got.Commits[0])
	}

	events, err := db.EventsByRepo(got.Commits[0].RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("dry-run stored events: %+v", events)
	}
	_, err = db.SyncState(got.Commits[0].RepoPath)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("dry-run cursor error = %v, want ErrNotFound", err)
	}
}

func TestCollectGitPersistsThenRespectsCursor(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")
	now := time.Date(2026, 11, 3, 15, 0, 0, 0, time.FixedZone("CST", -6*3600))
	when := now.Add(-2 * time.Minute)
	gitCommit(t, repo, when.Format(time.RFC3339), "me@example.com", "Me", "record me", "body")

	db := openScanDB(t)
	first, err := CollectGit(db, []string{root, root}, []string{"me@example.com"}, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Commits) != 1 || first.Inserted != 1 || first.Skipped != 0 {
		t.Fatalf("first collect = %+v, want 1 inserted", first)
	}
	repoPath := first.Commits[0].RepoPath
	events, err := db.EventsByRepo(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Hash != first.Commits[0].Hash || events[0].Subject != "record me" {
		t.Fatalf("events = %+v", events)
	}
	if events[0].ProjectSlug != "" {
		t.Fatalf("project slug = %q, want empty until project detection", events[0].ProjectSlug)
	}
	state, err := db.SyncState(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Cursor != now.Format(time.RFC3339) {
		t.Fatalf("cursor = %q, want %q", state.Cursor, now.Format(time.RFC3339))
	}

	second, err := CollectGit(db, []string{root}, []string{"me@example.com"}, now.Add(time.Second), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Commits) != 0 || second.Inserted != 0 {
		t.Fatalf("second collect = %+v, want cursor to hide the recorded commit", second)
	}
	events, err = db.EventsByRepo(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events after re-run = %d, want 1", len(events))
	}
}

func TestCollectGitRequiresAuthorsAndRoots(t *testing.T) {
	db := openScanDB(t)
	now := time.Now()
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")

	if _, err := CollectGit(db, nil, []string{"me@example.com"}, now, true); err == nil {
		t.Fatal("empty roots: error = nil")
	}
	if _, err := CollectGit(db, []string{root}, nil, now, false); err == nil {
		t.Fatal("missing authors: error = nil")
	}
	resolved, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	state, err := db.SyncState(resolved)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("failed collect cursor = %+v, err %v; want no write", state, err)
	}
}
