package scan

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestRepoSinceUsesLocalMidnightWhenMissing(t *testing.T) {
	db := openScanDB(t)
	now := time.Date(2026, 11, 3, 15, 4, 5, 0, time.FixedZone("CST", -6*3600))
	repo := "/Users/hackastak/Developer/Hackatrak"

	got, err := RepoSince(db, "  "+repo+"  ", now)
	if err != nil {
		t.Fatal(err)
	}
	want := LocalMidnight(now).Format(time.RFC3339)
	if got != want {
		t.Fatalf("RepoSince() = %q, want local midnight %q", got, want)
	}

	_, err = db.SyncState(repo)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SyncState error = %v, want ErrNotFound (read must not write)", err)
	}
}

func TestRepoSinceReturnsStoredCursor(t *testing.T) {
	db := openScanDB(t)
	repo := "/repo"
	now := time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)
	cursor := "2026-11-03T09:00:00-06:00"
	if _, err := db.SetSyncCursor(repo, cursor); err != nil {
		t.Fatal(err)
	}

	got, err := RepoSince(db, repo, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != cursor {
		t.Fatalf("RepoSince() = %q, want stored cursor %q", got, cursor)
	}
}

func TestAdvanceRepoCursorReplacesPerRepo(t *testing.T) {
	db := openScanDB(t)
	repo := "/repo"
	other := "/other"
	loc := time.FixedZone("CST", -6*3600)
	firstAt := time.Date(2026, 11, 3, 12, 0, 0, 900_000_000, loc)
	laterAt := time.Date(2026, 11, 3, 23, 0, 0, 0, loc)

	if _, err := db.SetSyncCursor(other, "leave-me"); err != nil {
		t.Fatal(err)
	}

	first, err := AdvanceRepoCursor(db, " "+repo+" ", firstAt)
	if err != nil {
		t.Fatal(err)
	}
	if first.RepoPath != repo || first.Cursor != firstAt.Format(time.RFC3339) {
		t.Fatalf("first advance = %+v, want path %q cursor %q", first, repo, firstAt.Format(time.RFC3339))
	}
	if strings.Contains(first.Cursor, ".") {
		t.Fatalf("cursor = %q, want second resolution", first.Cursor)
	}

	stored, err := db.SyncState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Cursor != first.Cursor {
		t.Fatalf("stored cursor = %q, want %q", stored.Cursor, first.Cursor)
	}

	advanced, err := AdvanceRepoCursor(db, repo, laterAt)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Cursor != laterAt.Format(time.RFC3339) {
		t.Fatalf("advanced cursor = %q", advanced.Cursor)
	}
	again, err := RepoSince(db, repo, laterAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if again != advanced.Cursor {
		t.Fatalf("RepoSince() = %q, want advanced cursor %q", again, advanced.Cursor)
	}

	left, err := db.SyncState(other)
	if err != nil {
		t.Fatal(err)
	}
	if left.Cursor != "leave-me" {
		t.Fatalf("other repo cursor = %q, want leave-me", left.Cursor)
	}
}

func TestRepoCursorIsRespectedByGitLog(t *testing.T) {
	db := openScanDB(t)
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")

	loc := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, 11, 3, 18, 0, 0, 0, loc)
	morning := time.Date(2026, 11, 3, 9, 0, 0, 0, loc)
	afternoon := time.Date(2026, 11, 3, 14, 0, 0, 0, loc)
	authors := []string{"me@example.com"}

	gitCommit(t, repo, morning.Add(-24*time.Hour).Format(time.RFC3339), "me@example.com", "Me", "yesterday", "")
	gitCommit(t, repo, morning.Format(time.RFC3339), "me@example.com", "Me", "morning", "")
	gitCommit(t, repo, afternoon.Format(time.RFC3339), "me@example.com", "Me", "afternoon", "")

	since, err := RepoSince(db, repo, now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := CommitsSince(repo, since, authors)
	if err != nil {
		t.Fatal(err)
	}
	if subjects(first) != "morning,afternoon" {
		t.Fatalf("first window = %q, want morning and afternoon", subjects(first))
	}

	if _, err := AdvanceRepoCursor(db, repo, morning.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	next, err := RepoSince(db, repo, now)
	if err != nil {
		t.Fatal(err)
	}
	if next != morning.Add(time.Second).Format(time.RFC3339) {
		t.Fatalf("advanced since = %q", next)
	}
	second, err := CommitsSince(repo, next, authors)
	if err != nil {
		t.Fatal(err)
	}
	if subjects(second) != "afternoon" {
		t.Fatalf("second window = %q, want only afternoon", subjects(second))
	}
}

func TestRepoCursorRejectsIncomplete(t *testing.T) {
	db := openScanDB(t)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	if _, err := RepoSince(nil, "/repo", now); err == nil {
		t.Fatal("RepoSince(nil db) error = nil")
	}
	if _, err := RepoSince(db, "  ", now); err == nil {
		t.Fatal("RepoSince(empty repo) error = nil")
	}
	if _, err := RepoSince(db, "/repo", time.Time{}); err == nil {
		t.Fatal("RepoSince(zero time) error = nil")
	}
	if _, err := AdvanceRepoCursor(nil, "/repo", now); err == nil {
		t.Fatal("AdvanceRepoCursor(nil db) error = nil")
	}
	if _, err := AdvanceRepoCursor(db, "", now); err == nil {
		t.Fatal("AdvanceRepoCursor(empty repo) error = nil")
	}
	if _, err := AdvanceRepoCursor(db, "/repo", time.Time{}); err == nil {
		t.Fatal("AdvanceRepoCursor(zero time) error = nil")
	}

	_, err := db.SyncState("/repo")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("rejected advance wrote a cursor: %v", err)
	}
}

func TestRepoCursorSurfacesStoreErrors(t *testing.T) {
	db := openScanDB(t)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := RepoSince(db, "/repo", now); err == nil {
		t.Fatal("RepoSince(closed db) error = nil")
	}
	if _, err := AdvanceRepoCursor(db, "/repo", now); err == nil {
		t.Fatal("AdvanceRepoCursor(closed db) error = nil")
	}
}

func subjects(commits []Commit) string {
	names := make([]string, len(commits))
	for i, commit := range commits {
		names[i] = commit.Subject
	}
	return strings.Join(names, ",")
}
