package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestInsertEventRoundTrip(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 10, 28, 15, 4, 5, 0, time.UTC)

	inserted, err := db.InsertEvent(Event{
		Kind:        KindCommit,
		Hash:        "abc123",
		RepoPath:    "/Users/hackastak/Developer/Hackatrak",
		ProjectSlug: "Hackatrak",
		OccurredAt:  when,
		Branch:      "main",
		Subject:     "feat: open the store",
		Body:        "schema v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted.ID == 0 {
		t.Fatal("InsertEvent ID = 0, want a row id")
	}

	got, err := db.EventByHash("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != inserted.ID || got.Kind != KindCommit || got.Subject != "feat: open the store" {
		t.Fatalf("EventByHash = %+v", got)
	}
	if !got.OccurredAt.Equal(when) {
		t.Fatalf("OccurredAt = %s, want %s", got.OccurredAt, when)
	}
	if got.Body != "schema v1" || got.Branch != "main" || got.ProjectSlug != "Hackatrak" {
		t.Fatalf("EventByHash = %+v", got)
	}
}

func TestInsertEventRejectsDuplicateHash(t *testing.T) {
	db := openTestDB(t)
	event := Event{
		Kind:       KindCommit,
		Hash:       "abc123",
		RepoPath:   "/repo",
		OccurredAt: time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC),
	}
	if _, err := db.InsertEvent(event); err != nil {
		t.Fatal(err)
	}
	_, err := db.InsertEvent(event)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second InsertEvent error = %v, want ErrDuplicate", err)
	}
}

func TestEventByHashNotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.EventByHash("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("EventByHash error = %v, want ErrNotFound", err)
	}
}

func TestEventsByRepoOrdersByTime(t *testing.T) {
	db := openTestDB(t)
	repo := "/Users/hackastak/Developer/Hackatrak"
	other := "/Users/hackastak/Developer/other"
	early := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC)

	if _, err := db.InsertEvent(Event{
		Kind: KindFileTouch, RepoPath: repo, OccurredAt: late, FileCount: 2, FileSample: "a.go, b.go",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertEvent(Event{
		Kind: KindCommit, Hash: "late", RepoPath: other, OccurredAt: late, Subject: "other",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertEvent(Event{
		Kind: KindCommit, Hash: "early", RepoPath: repo, OccurredAt: early, Subject: "first",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := db.EventsByRepo(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Hash != "early" || got[1].Kind != KindFileTouch || got[1].FileCount != 2 || got[1].FileSample != "a.go, b.go" {
		t.Fatalf("EventsByRepo = %+v", got)
	}
}

func TestEventsByRepoRejectsEmptyPath(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.EventsByRepo(""); err == nil {
		t.Fatal("EventsByRepo(empty) error = nil, want an error")
	}
	if _, err := db.EventByHash(""); err == nil {
		t.Fatal("EventByHash(empty) error = nil, want an error")
	}
}

func TestFileTouchEventsDoNotCollide(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if _, err := db.InsertEvent(Event{
			Kind: KindFileTouch, RepoPath: "/repo", OccurredAt: when, FileCount: i + 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.EventsByRepo("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestDeleteFileTouchRemovesOnlyThatRow(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	touch, err := db.InsertEvent(Event{
		Kind: KindFileTouch, RepoPath: "/repo", OccurredAt: when, FileCount: 1, FileSample: "a.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := db.InsertEvent(Event{
		Kind: KindCommit, Hash: "abc", RepoPath: "/repo", OccurredAt: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteFileTouch(0); err == nil {
		t.Fatal("id 0: error = nil, want an error")
	}
	if err := db.DeleteFileTouch(commit.ID); err == nil {
		t.Fatal("commit id: error = nil, want an error")
	}
	if err := db.DeleteFileTouch(touch.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteFileTouch(touch.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v, want ErrNotFound", err)
	}
	stored, err := db.EventsByRepo("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Kind != KindCommit {
		t.Fatalf("stored = %+v, want the commit", stored)
	}
}

func TestUpdateFileTouchReplacesSample(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.FixedZone("CST", -6*3600))
	inserted, err := db.InsertEvent(Event{
		Kind: KindFileTouch, RepoPath: "/repo", ProjectSlug: "repo", OccurredAt: when, FileCount: 1, FileSample: "a.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	later := when.Add(time.Hour)
	updated, err := db.UpdateFileTouch(inserted.ID, later, 3, "a.go, b.go", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != inserted.ID || updated.Kind != KindFileTouch || updated.Hash != "" || updated.RepoPath != "/repo" {
		t.Fatalf("updated identity = %+v", updated)
	}
	if updated.FileCount != 3 || updated.FileSample != "a.go, b.go" || updated.ProjectSlug != "repo" {
		t.Fatalf("updated fields = %+v", updated)
	}
	if !updated.OccurredAt.Equal(later) {
		t.Fatalf("OccurredAt = %s, want %s", updated.OccurredAt, later)
	}
}

func TestUpdateFileTouchRejectsMissingAndCommits(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	commit, err := db.InsertEvent(Event{
		Kind: KindCommit, Hash: "abc", RepoPath: "/repo", OccurredAt: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateFileTouch(0, when, 1, "a.go", ""); err == nil {
		t.Fatal("id 0: error = nil, want an error")
	}
	if _, err := db.UpdateFileTouch(commit.ID, time.Time{}, 1, "a.go", ""); err == nil {
		t.Fatal("zero time: error = nil, want an error")
	}
	if _, err := db.UpdateFileTouch(commit.ID, when, 1, "a.go", ""); err == nil {
		t.Fatal("commit id: error = nil, want an error")
	}
	_, err = db.UpdateFileTouch(99, when, 1, "a.go", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing id error = %v, want ErrNotFound", err)
	}
	stored, err := db.EventByHash("abc")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Kind != KindCommit || stored.Hash != "abc" {
		t.Fatalf("commit changed: %+v", stored)
	}
}

func TestInsertEventRejectsIncomplete(t *testing.T) {
	db := openTestDB(t)
	when := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)

	cases := []Event{
		{Hash: "x", RepoPath: "/repo", OccurredAt: when},
		{Kind: KindCommit, RepoPath: "/repo", OccurredAt: when},
		{Kind: KindCommit, Hash: "x", OccurredAt: when},
		{Kind: KindFileTouch, RepoPath: "/repo"},
		{Kind: "note", RepoPath: "/repo", OccurredAt: when},
		{Kind: KindFileTouch, Hash: "abc", RepoPath: "/repo", OccurredAt: when},
	}
	for _, event := range cases {
		if _, err := db.InsertEvent(event); err == nil {
			t.Fatalf("InsertEvent(%+v) error = nil, want an error", event)
		}
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "hackatrak.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
