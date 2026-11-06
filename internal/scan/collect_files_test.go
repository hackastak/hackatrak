package scan

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestCollectFilesRecordsPresenceWithoutACommit(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, "main.go"), now.Add(-time.Hour))
	writeFileAt(t, filepath.Join(root, "notes.md"), now.Add(-30*time.Minute))
	writeFileAt(t, filepath.Join(repo, "old.go"), now.Add(-48*time.Hour))

	db := openScanDB(t)
	got, err := CollectFiles(db, []string{root}, nil, now, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Inserted != 2 || got.Updated != 0 || len(got.Touches) != 2 {
		t.Fatalf("collect = %+v, want a repo touch and a root touch", got)
	}

	repoPath := resolved(t, repo)
	repoEvents, err := db.EventsByRepo(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(repoEvents) != 1 || repoEvents[0].Kind != store.KindFileTouch || repoEvents[0].FileCount != 1 || repoEvents[0].FileSample != "main.go" {
		t.Fatalf("repo events = %+v", repoEvents)
	}
	rootEvents, err := db.EventsByRepo(resolved(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if len(rootEvents) != 1 || rootEvents[0].FileSample != "notes.md" {
		t.Fatalf("root events = %+v", rootEvents)
	}
}

func TestCollectFilesDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, "main.go"), now.Add(-time.Minute))

	db := openScanDB(t)
	got, err := CollectFiles(db, []string{root}, nil, now, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Touches) != 1 || got.Inserted != 0 || got.Updated != 0 || got.Removed != 0 {
		t.Fatalf("dry-run = %+v", got)
	}
	events, err := db.EventsByRepo(resolved(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("dry-run stored %+v", events)
	}
}

func TestCollectFilesDoesNotDoubleCountGit(t *testing.T) {
	root := t.TempDir()
	committed := filepath.Join(root, "shipped")
	fallback := filepath.Join(root, "dirty")
	writeFileAt(t, filepath.Join(committed, ".git", "HEAD"), time.Now())
	writeFileAt(t, filepath.Join(fallback, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(committed, "main.go"), now.Add(-time.Minute))
	writeFileAt(t, filepath.Join(fallback, "notes.md"), now.Add(-time.Minute))

	db := openScanDB(t)
	committedPath := resolved(t, committed)
	if _, err := db.InsertEvent(store.Event{
		Kind:       store.KindFileTouch,
		RepoPath:   committedPath,
		OccurredAt: now.Add(-2 * time.Hour),
		FileCount:  1,
		FileSample: "stale.go",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertEvent(store.Event{
		Kind:       store.KindCommit,
		Hash:       "abc123",
		RepoPath:   committedPath,
		OccurredAt: now.Add(-time.Hour),
		Subject:    "ship it",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := CollectFiles(db, []string{root}, nil, now, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Inserted != 1 || got.Removed != 1 || len(got.Touches) != 1 {
		t.Fatalf("collect = %+v, want fallback only and the stale touch removed", got)
	}
	if got.Touches[0].Root != resolved(t, fallback) {
		t.Fatalf("touch = %+v, want the repo without a commit", got.Touches[0])
	}

	shipped, err := db.EventsByRepo(committedPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(shipped) != 1 || shipped[0].Kind != store.KindCommit {
		t.Fatalf("shipped events = %+v, want the commit only", shipped)
	}
	dirty, err := db.EventsByRepo(resolved(t, fallback))
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) != 1 || dirty[0].Kind != store.KindFileTouch {
		t.Fatalf("fallback events = %+v", dirty)
	}
}

func TestCollectFilesSkipsReposCommittedThisPass(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, "main.go"), now.Add(-time.Minute))

	db := openScanDB(t)
	repoPath := resolved(t, repo)
	got, err := CollectFiles(db, []string{root}, nil, now, true, []string{repoPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Touches) != 0 || got.Inserted != 0 {
		t.Fatalf("collect = %+v, want no file presence beside this pass's commits", got)
	}
}

func TestCollectFilesReplacesSameDayTouch(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, "a.go"), now.Add(-2*time.Hour))

	db := openScanDB(t)
	if _, err := CollectFiles(db, []string{root}, nil, now, false, nil); err != nil {
		t.Fatal(err)
	}
	writeFileAt(t, filepath.Join(repo, "b.go"), now.Add(-time.Minute))
	second, err := CollectFiles(db, []string{root}, nil, now.Add(time.Minute), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Inserted != 0 || second.Updated != 1 {
		t.Fatalf("second = %+v, want an update", second)
	}
	events, err := db.EventsByRepo(resolved(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].FileCount != 2 || events[0].FileSample != "a.go, b.go" {
		t.Fatalf("events = %+v", events)
	}
}

func TestCollectFilesKeepsPresenceAfterYesterdaysCommit(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), time.Now())
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, "main.go"), now.Add(-time.Minute))

	db := openScanDB(t)
	repoPath := resolved(t, repo)
	if _, err := db.InsertEvent(store.Event{
		Kind:       store.KindCommit,
		Hash:       "old",
		RepoPath:   repoPath,
		OccurredAt: now.Add(-24 * time.Hour),
		Subject:    "yesterday",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := CollectFiles(db, []string{root}, nil, now, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Inserted != 1 || len(got.Touches) != 1 {
		t.Fatalf("collect = %+v, want today's file presence", got)
	}
	events, err := db.EventsByRepo(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want the old commit and today's file-touch", events)
	}
}

func TestCollectFilesIgnoresNoise(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(repo, ".git", "HEAD"), now)
	writeFileAt(t, filepath.Join(repo, ".DS_Store"), now)
	writeFileAt(t, filepath.Join(repo, "dist", "app.js"), now)
	writeFileAt(t, filepath.Join(repo, "notes.txt~"), now)

	db := openScanDB(t)
	got, err := CollectFiles(db, []string{root}, nil, now, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Touches) != 0 || got.Inserted != 0 {
		t.Fatalf("noise collect = %+v, want no presence", got)
	}
}
