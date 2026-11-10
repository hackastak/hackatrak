package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestSummaryDefaultsToToday(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)
	db := openCLIStore(t, home)

	now := time.Now().In(time.Local)
	today := now.Format("2006-01-02")
	other := now.AddDate(0, 0, -8).Format("2006-01-02")
	seedCLIEvent(t, db, store.Event{
		Kind:        store.KindCommit,
		Hash:        "sum1",
		RepoPath:    home,
		ProjectSlug: "staksmith",
		OccurredAt:  now,
		Branch:      "main",
		Subject:     "rework docs flow",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug:  "staksmith",
		Day:          today,
		Worked:       true,
		Source:       store.PresenceCommit,
		Branch:       "main",
		Ticket:       "NEXTGEN-1",
		CommitHashes: []string{"sum1"},
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "notes",
		Day:         other,
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "old.md",
	})

	out, err := execute(t, "summary")
	if err != nil {
		t.Fatalf("summary: %v\n%s", err, out)
	}
	if strings.Contains(out, "not implemented") || strings.Contains(out, "notes") || strings.Contains(out, other) {
		t.Fatalf("summary is not a today rollup:\n%s", out)
	}
	for _, want := range []string{today, "staksmith", "NEXTGEN-1", "rework docs flow", "1 project\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestSummaryWeekRollup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)
	db := openCLIStore(t, home)

	now := time.Now().In(time.Local)
	monday := weekMonday(now)
	outside := monday.AddDate(0, 0, -1)
	later := monday.AddDate(0, 0, 1)
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "notes",
		Day:         monday.Format("2006-01-02"),
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "a.md",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "staksmith",
		Day:         later.Format("2006-01-02"),
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   2,
		FileSample:  "a.go, b.go",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "outside",
		Day:         outside.Format("2006-01-02"),
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "nope.md",
	})

	out, err := execute(t, "summary", "--week")
	if err != nil {
		t.Fatalf("summary --week: %v\n%s", err, out)
	}
	year, week := monday.ISOWeek()
	wantWeek := fmt.Sprintf("%04d-W%02d", year, week)
	if !strings.Contains(out, wantWeek) {
		t.Fatalf("summary --week missing %s:\n%s", wantWeek, out)
	}
	if strings.Contains(out, "outside") || strings.Contains(out, outside.Format("2006-01-02")) {
		t.Fatalf("summary --week included the previous week:\n%s", out)
	}
	for _, want := range []string{
		monday.Format("2006-01-02"),
		later.Format("2006-01-02"),
		"notes",
		"touched 1 file (a.md)",
		"staksmith",
		"touched 2 files (a.go, b.go)",
		"2 projects over 2 days\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary --week missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, monday.Format("2006-01-02")) > strings.Index(out, later.Format("2006-01-02")) {
		t.Fatalf("days out of order:\n%s", out)
	}
}

func TestSummaryEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "summary")
	if err != nil {
		t.Fatalf("summary: %v\n%s", err, out)
	}
	if out != "summary: no work\n" {
		t.Fatalf("output = %q", out)
	}
	out, err = execute(t, "summary", "--week")
	if err != nil {
		t.Fatalf("summary --week: %v\n%s", err, out)
	}
	if out != "summary: no work\n" {
		t.Fatalf("week output = %q", out)
	}
}
