package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestTodayJSONMatchesQuery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)
	db := openCLIStore(t, home)

	now := time.Now().In(time.Local)
	today := now.Format("2006-01-02")
	other := now.AddDate(0, 0, -8).Format("2006-01-02")
	seedCLIEvent(t, db, store.Event{
		Kind:        store.KindCommit,
		Hash:        "json1",
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
		CommitHashes: []string{"json1"},
		FileCount:    1,
		FileSample:   "docs.go",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "notes",
		Day:         other,
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "old.md",
	})

	out, err := execute(t, "today", "--json")
	if err != nil {
		t.Fatalf("today --json: %v\n%s", err, out)
	}
	if strings.Contains(out, "PROJECT") || strings.Contains(out, "notes") {
		t.Fatalf("today --json is not query JSON:\n%s", out)
	}
	doc := decodeReview(t, out)
	if doc.Period != "day" || doc.Date != today || doc.Week != "" {
		t.Fatalf("doc = %+v, want day %s", doc, today)
	}
	if doc.ProjectCount != 1 || doc.DayCount != 1 || len(doc.Projects) != 1 {
		t.Fatalf("doc = %+v", doc)
	}
	row := doc.Projects[0]
	if row.Day != today || row.Project != "staksmith" || row.Source != "commit" || row.Branch != "main" || row.Ticket != "NEXTGEN-1" || row.Work != "rework docs flow" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSummaryJSONWeek(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)
	db := openCLIStore(t, home)

	now := time.Now().In(time.Local)
	monday := weekMonday(now)
	later := monday.AddDate(0, 0, 2)
	outside := monday.AddDate(0, 0, -1)
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
		FileCount:   1,
		FileSample:  "a.go",
	})
	seedCLIPresence(t, db, store.DayPresence{
		ProjectSlug: "outside",
		Day:         outside.Format("2006-01-02"),
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   1,
		FileSample:  "nope.md",
	})

	out, err := execute(t, "summary", "--week", "--json")
	if err != nil {
		t.Fatalf("summary --week --json: %v\n%s", err, out)
	}
	doc := decodeReview(t, out)
	year, week := monday.ISOWeek()
	wantWeek := sprintfWeek(year, week)
	if doc.Period != "week" || doc.Week != wantWeek || doc.Date != "" {
		t.Fatalf("header = %+v, want week %s", doc.reviewHeader, wantWeek)
	}
	if doc.ProjectCount != 2 || doc.DayCount != 2 || len(doc.Projects) != 2 {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.Projects[0].Project != "notes" || doc.Projects[0].Work != "touched 1 file (a.md)" {
		t.Fatalf("first = %+v", doc.Projects[0])
	}
	if doc.Projects[1].Project != "staksmith" || doc.Projects[1].Day != later.Format("2006-01-02") {
		t.Fatalf("second = %+v", doc.Projects[1])
	}
}

func TestJSONEmptyIsAnArray(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "today", "--json")
	if err != nil {
		t.Fatalf("today --json: %v\n%s", err, out)
	}
	doc := decodeReview(t, out)
	today := time.Now().In(time.Local).Format("2006-01-02")
	if doc.Period != "day" || doc.Date != today || doc.Projects == nil || len(doc.Projects) != 0 || doc.ProjectCount != 0 {
		t.Fatalf("empty today = %+v", doc)
	}
	if !strings.Contains(out, `"projects": []`) {
		t.Fatalf("empty projects encoded as null:\n%s", out)
	}

	out, err = execute(t, "summary", "--week", "--json")
	if err != nil {
		t.Fatalf("summary --json: %v\n%s", err, out)
	}
	doc = decodeReview(t, out)
	year, week := time.Now().In(time.Local).ISOWeek()
	if doc.Period != "week" || doc.Week != sprintfWeek(year, week) || len(doc.Projects) != 0 {
		t.Fatalf("empty week = %+v", doc)
	}
}

type reviewHeader struct {
	Period       string `json:"period"`
	Date         string `json:"date"`
	Week         string `json:"week"`
	ProjectCount int    `json:"project_count"`
	DayCount     int    `json:"day_count"`
}

type reviewJSON struct {
	reviewHeader
	Projects []reviewJSONRow `json:"projects"`
}

type reviewJSONRow struct {
	Day     string `json:"day"`
	Project string `json:"project"`
	Bucket  string `json:"bucket"`
	Source  string `json:"source"`
	Branch  string `json:"branch"`
	Ticket  string `json:"ticket"`
	Work    string `json:"work"`
}

func decodeReview(t *testing.T, out string) reviewJSON {
	t.Helper()
	var doc reviewJSON
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return doc
}

func sprintfWeek(year, week int) string {
	return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006") + "-W" + pad2(week)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [4]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
