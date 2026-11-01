package store

import (
	"errors"
	"testing"
	"time"
)

func TestSyncCursorRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := "/Users/hackastak/Developer/Hackatrak"

	_, err := db.SyncState(repo)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("SyncState error = %v, want ErrNotFound", err)
	}

	first, err := db.SetSyncCursor(repo, "2026-10-28T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if first.RepoPath != repo || first.Cursor != "2026-10-28T12:00:00Z" || first.UpdatedAt.IsZero() {
		t.Fatalf("SetSyncCursor = %+v", first)
	}

	got, err := db.SyncState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cursor != first.Cursor || got.RepoPath != repo {
		t.Fatalf("SyncState = %+v, want cursor %q", got, first.Cursor)
	}

	time.Sleep(time.Millisecond)
	advanced, err := db.SetSyncCursor(repo, "2026-10-28T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Cursor != "2026-10-28T23:00:00Z" {
		t.Fatalf("advanced cursor = %q", advanced.Cursor)
	}
	if !advanced.UpdatedAt.After(first.UpdatedAt) && !advanced.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("UpdatedAt did not move: first %s advanced %s", first.UpdatedAt, advanced.UpdatedAt)
	}

	again, err := db.SyncState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if again.Cursor != "2026-10-28T23:00:00Z" {
		t.Fatalf("SyncState cursor = %q, want advanced cursor", again.Cursor)
	}
}

func TestSetSyncCursorRejectsEmpty(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.SyncState(""); err == nil {
		t.Fatal("SyncState(empty) error = nil, want an error")
	}
	if _, err := db.SetSyncCursor("", "cursor"); err == nil {
		t.Fatal("SetSyncCursor(empty repo) error = nil, want an error")
	}
	if _, err := db.SetSyncCursor("/repo", ""); err == nil {
		t.Fatal("SetSyncCursor(empty cursor) error = nil, want an error")
	}
}

func TestSyncCursorsArePerRepo(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.SetSyncCursor("/a", "a-cursor"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetSyncCursor("/b", "b-cursor"); err != nil {
		t.Fatal(err)
	}
	a, err := db.SyncState("/a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.SyncState("/b")
	if err != nil {
		t.Fatal(err)
	}
	if a.Cursor != "a-cursor" || b.Cursor != "b-cursor" {
		t.Fatalf("cursors = %q %q", a.Cursor, b.Cursor)
	}
}
