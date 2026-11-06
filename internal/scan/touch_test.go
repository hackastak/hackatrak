package scan

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestFileTouchesGroupsByRoot(t *testing.T) {
	early := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	late := early.Add(2 * time.Hour)
	other := early.Add(30 * time.Minute)
	files := []TouchedFile{
		{Path: "/work/repo/z.go", Root: "/work/repo", MTime: early},
		{Path: "/work/other/notes.md", Root: "/work/other", MTime: other},
		{Path: "/work/repo/src/a.go", Root: "/work/repo", MTime: late},
		{Path: "/work/repo/src/b.go", Root: "/work/repo", MTime: early.Add(time.Minute)},
		{Path: "/work/repo/src/c.go", Root: "/work/repo", MTime: early.Add(2 * time.Minute)},
		{Path: "/work/repo/nested/a.go", Root: "/work/repo", MTime: early.Add(3 * time.Minute)},
	}

	got, err := FileTouches(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Root != "/work/other" || got[0].Count != 1 || got[0].Sample != "notes.md" {
		t.Fatalf("other = %+v", got[0])
	}
	if !got[0].OccurredAt.Equal(other) {
		t.Fatalf("other time = %s, want %s", got[0].OccurredAt, other)
	}
	repo := got[1]
	if repo.Root != "/work/repo" || repo.Count != 5 || repo.Sample != "a.go, b.go, c.go" {
		t.Fatalf("repo = %+v, want count 5 and sample %q", repo, "a.go, b.go, c.go")
	}
	if !repo.OccurredAt.Equal(late) {
		t.Fatalf("repo time = %s, want latest %s", repo.OccurredAt, late)
	}
}

func TestFileTouchesEmpty(t *testing.T) {
	got, err := FileTouches(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestFileTouchesRejectsIncomplete(t *testing.T) {
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	cases := []TouchedFile{
		{Path: "/repo/a.go", MTime: when},
		{Root: "/repo", MTime: when},
		{Path: "/repo/a.go", Root: "/repo"},
	}
	for _, file := range cases {
		if _, err := FileTouches([]TouchedFile{file}); err == nil {
			t.Fatalf("FileTouches(%+v) error = nil, want an error", file)
		}
	}
}

func TestFileTouchesDoesNotMutateInput(t *testing.T) {
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	files := []TouchedFile{
		{Path: "/repo/b.go", Root: "/repo", MTime: when},
		{Path: "/repo/a.go", Root: "/repo", MTime: when.Add(time.Minute)},
	}
	before := append([]TouchedFile(nil), files...)
	if _, err := FileTouches(files); err != nil {
		t.Fatal(err)
	}
	if files[0] != before[0] || files[1] != before[1] {
		t.Fatalf("input mutated: got %+v, want %+v", files, before)
	}
}

func TestPersistFileTouchesEmitsPresenceEvent(t *testing.T) {
	db := openScanDB(t)
	when := time.Date(2026, 11, 5, 10, 15, 0, 0, time.Local)
	files := []TouchedFile{
		{Path: "/work/repo/src/main.go", Root: "/work/repo", MTime: when},
		{Path: "/work/repo/README.md", Root: "/work/repo", MTime: when.Add(-time.Hour)},
	}

	got, err := PersistFileTouches(db, "repo", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inserted) != 1 || len(got.Updated) != 0 {
		t.Fatalf("PersistFileTouches = %+v, want 1 inserted", got)
	}
	event := got.Inserted[0]
	if event.ID == 0 || event.Kind != store.KindFileTouch || event.Hash != "" {
		t.Fatalf("inserted = %+v", event)
	}
	if event.RepoPath != "/work/repo" || event.ProjectSlug != "repo" {
		t.Fatalf("path/slug = %+v", event)
	}
	if event.FileCount != 2 || event.FileSample != "README.md, main.go" {
		t.Fatalf("count/sample = %d %q", event.FileCount, event.FileSample)
	}
	if !event.OccurredAt.Equal(when) {
		t.Fatalf("OccurredAt = %s, want %s", event.OccurredAt, when)
	}

	stored, err := db.EventsByRepo("/work/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].ID != event.ID {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestPersistFileTouchesReplacesSameLocalDay(t *testing.T) {
	db := openScanDB(t)
	day := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	firstFiles := []TouchedFile{
		{Path: "/work/repo/a.go", Root: "/work/repo", MTime: day},
	}
	first, err := PersistFileTouches(db, "repo", firstFiles)
	if err != nil {
		t.Fatal(err)
	}

	later := day.Add(4 * time.Hour)
	second, err := PersistFileTouches(db, "", []TouchedFile{
		{Path: "/work/repo/a.go", Root: "/work/repo", MTime: day},
		{Path: "/work/repo/b.go", Root: "/work/repo", MTime: later},
		{Path: "/work/repo/c.go", Root: "/work/repo", MTime: later},
		{Path: "/work/repo/d.go", Root: "/work/repo", MTime: later},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Inserted) != 0 || len(second.Updated) != 1 {
		t.Fatalf("second persist = %+v, want 1 updated", second)
	}
	updated := second.Updated[0]
	if updated.ID != first.Inserted[0].ID {
		t.Fatalf("updated id = %d, want %d", updated.ID, first.Inserted[0].ID)
	}
	if updated.FileCount != 4 || updated.FileSample != "a.go, b.go, c.go" {
		t.Fatalf("updated count/sample = %d %q", updated.FileCount, updated.FileSample)
	}
	if updated.ProjectSlug != "repo" {
		t.Fatalf("slug = %q, want existing slug kept", updated.ProjectSlug)
	}
	if !updated.OccurredAt.Equal(later) {
		t.Fatalf("OccurredAt = %s, want %s", updated.OccurredAt, later)
	}

	stored, err := db.EventsByRepo("/work/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d events, want 1: %+v", len(stored), stored)
	}
}

func TestPersistFileTouchesKeepsOtherDaysAndCommits(t *testing.T) {
	db := openScanDB(t)
	today := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	yesterday := today.Add(-24 * time.Hour)
	if _, err := db.InsertEvent(store.Event{
		Kind:       store.KindFileTouch,
		RepoPath:   "/work/repo",
		OccurredAt: yesterday,
		FileCount:  1,
		FileSample: "old.go",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertEvent(store.Event{
		Kind:       store.KindCommit,
		Hash:       "abc123",
		RepoPath:   "/work/repo",
		OccurredAt: today,
		Subject:    "feat: keep me",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := PersistFileTouches(db, "", []TouchedFile{
		{Path: "/work/repo/main.go", Root: "/work/repo", MTime: today},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inserted) != 1 || len(got.Updated) != 0 {
		t.Fatalf("persist = %+v, want a new file-touch", got)
	}

	stored, err := db.EventsByRepo("/work/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 3 {
		t.Fatalf("stored %d events, want yesterday + commit + today: %+v", len(stored), stored)
	}
	if stored[1].Kind != store.KindCommit || stored[1].Hash != "abc123" {
		t.Fatalf("commit changed: %+v", stored[1])
	}
}

func TestPersistFileTouchesEmptyIsNoop(t *testing.T) {
	db := openScanDB(t)
	got, err := PersistFileTouches(db, "repo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inserted) != 0 || len(got.Updated) != 0 {
		t.Fatalf("persist = %+v, want nothing", got)
	}
}

func TestPersistFileTouchesRejectsIncompleteBeforeWrite(t *testing.T) {
	db := openScanDB(t)
	when := time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)
	_, err := PersistFileTouches(db, "", []TouchedFile{
		{Path: "/work/ok/a.go", Root: "/work/ok", MTime: when},
		{Path: "/work/bad/a.go", Root: "/work/bad"},
	})
	if err == nil {
		t.Fatal("error = nil, want an error")
	}
	stored, err := db.EventsByRepo("/work/ok")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("partial write: %+v", stored)
	}
	if _, err := PersistFileTouches(nil, "", []TouchedFile{
		{Path: filepath.Join("/repo", "a.go"), Root: "/repo", MTime: when},
	}); err == nil {
		t.Fatal("nil db: error = nil, want an error")
	}
}

func TestPersistFileTouchesSurfacesStoreErrors(t *testing.T) {
	db := openScanDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := PersistFileTouches(db, "", []TouchedFile{
		{Path: "/repo/a.go", Root: "/repo", MTime: time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local)},
	})
	if err == nil {
		t.Fatal("closed db: error = nil, want an error")
	}
	if errors.Is(err, store.ErrDuplicate) {
		t.Fatal("closed db reported a duplicate, want a store error")
	}
}
