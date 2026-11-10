package main

import (
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestTodayPrintsQueryTable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)
	db := openCLIStore(t, home)

	today := time.Now().In(time.Local).Format("2006-01-02")
	yesterday := time.Now().In(time.Local).AddDate(0, 0, -1).Format("2006-01-02")
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug:  "staksmith",
		Day:          today,
		Worked:       true,
		Source:       store.PresenceCommit,
		Branch:       "main",
		Ticket:       "NEXTGEN-1",
		CommitHashes: []string{"aaa"},
		FileCount:    1,
		FileSample:   "docs.go",
	})
	seedCLIEvent(t, db, store.Event{
		Kind:        store.KindCommit,
		Hash:        "aaa",
		RepoPath:    home,
		ProjectSlug: "staksmith",
		OccurredAt:  time.Now(),
		Branch:      "main",
		Subject:     "rework docs flow",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "notes",
		Day:         today,
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   2,
		FileSample:  "a.md, b.md",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "other",
		Day:         yesterday,
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "old.md",
	})

	out, err := execute(t, "today")
	if err != nil {
		t.Fatalf("today: %v\n%s", err, out)
	}
	if strings.Contains(out, "not implemented") {
		t.Fatalf("today is still a stub:\n%s", out)
	}
	if strings.Contains(out, "other") || strings.Contains(out, yesterday) {
		t.Fatalf("today included another day:\n%s", out)
	}
	if !strings.Contains(out, today) {
		t.Fatalf("today output missing date %s:\n%s", today, out)
	}
	for _, want := range []string{
		"PROJECT", "BUCKET", "SOURCE", "BRANCH", "TICKET", "WORK",
		"notes", "file", "touched 2 files (a.md, b.md)",
		"staksmith", "commit", "main", "NEXTGEN-1", "rework docs flow",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("today output missing %q:\n%s", want, out)
		}
	}
	notesAt := strings.Index(out, "notes")
	stakAt := strings.Index(out, "staksmith")
	if notesAt < 0 || stakAt < 0 || notesAt > stakAt {
		t.Fatalf("rows not ordered by project:\n%s", out)
	}
}

func TestTodayEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "today")
	if err != nil {
		t.Fatalf("today: %v\n%s", err, out)
	}
	if out != "today: no work\n" {
		t.Fatalf("output = %q, want %q", out, "today: no work\n")
	}
}

func seedCLIPresence(t *testing.T, db *store.DB, row store.DayPresence) {
	t.Helper()
	if _, err := db.UpsertDayPresence(row); err != nil {
		t.Fatal(err)
	}
}

func seedCLIEvent(t *testing.T, db *store.DB, event store.Event) {
	t.Helper()
	if _, err := db.InsertEvent(event); err != nil {
		t.Fatal(err)
	}
}
