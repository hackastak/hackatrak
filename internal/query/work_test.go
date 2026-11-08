package query

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestTodaysWorkUsesLocalDay(t *testing.T) {
	db := openQueryDB(t)
	seedPresence(t, db, store.DayPresence{
		ProjectSlug:  "staksmith",
		Day:          "2026-11-06",
		Worked:       true,
		Source:       store.PresenceCommit,
		Branch:       "main",
		Ticket:       "NEXTGEN-1",
		CommitHashes: []string{"bbb", "aaa"},
		FileCount:    1,
		FileSample:   "keep.go",
	})
	seedPresence(t, db, store.DayPresence{
		ProjectSlug: "notes",
		Day:         "2026-11-06",
		Worked:      true,
		Source:      store.PresenceFile,
		FileCount:   3,
		FileSample:  "a.md, b.md",
	})
	seedPresence(t, db, store.DayPresence{
		ProjectSlug:  "hackatrak",
		Day:          "2026-11-07",
		Worked:       true,
		Source:       store.PresenceCommit,
		CommitHashes: []string{"ccc"},
		Branch:       "feat/p4",
	})
	seedCommit(t, db, "bbb", "feat/old", "second", "body")
	seedCommit(t, db, "aaa", "main", "first", "")

	loc := time.FixedZone("CST", -6*60*60)
	q := mustQuery(t, db, loc)
	// 05:30 UTC is still the 6th in CST. UTC grouping would pick the 7th.
	q.now = func() time.Time {
		return time.Date(2026, 11, 7, 5, 30, 0, 0, time.UTC)
	}

	got, err := q.TodaysWork()
	if err != nil {
		t.Fatal(err)
	}
	want := []ProjectDay{
		{
			Slug:       "notes",
			Day:        Date{Year: 2026, Month: time.November, Day: 6},
			Worked:     true,
			Source:     SourceFile,
			FileCount:  3,
			FileSample: "a.md, b.md",
		},
		{
			Slug:   "staksmith",
			Day:    Date{Year: 2026, Month: time.November, Day: 6},
			Worked: true,
			Source: SourceCommit,
			Branch: "main",
			Ticket: "NEXTGEN-1",
			Commits: []Commit{
				{Hash: "bbb", Branch: "feat/old", Subject: "second", Body: "body"},
				{Hash: "aaa", Branch: "main", Subject: "first"},
			},
			FileCount:  1,
			FileSample: "keep.go",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TodaysWork() = %#v, want %#v", got, want)
	}
}

func TestTodaysWorkEmpty(t *testing.T) {
	q := mustQuery(t, openQueryDB(t), time.UTC)
	q.now = func() time.Time { return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC) }
	got, err := q.TodaysWork()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("TodaysWork() = %#v, want nil", got)
	}
}

func TestWeekWorkISOBounds(t *testing.T) {
	db := openQueryDB(t)
	for _, day := range []string{"2025-12-28", "2025-12-29", "2026-01-04", "2026-01-05"} {
		seedPresence(t, db, store.DayPresence{
			ProjectSlug: "hackatrak",
			Day:         day,
			Worked:      true,
			Source:      store.PresenceFile,
			FileCount:   1,
			FileSample:  day + ".go",
		})
	}
	seedPresence(t, db, store.DayPresence{
		ProjectSlug:  "staksmith",
		Day:          "2026-01-04",
		Worked:       true,
		Source:       store.PresenceCommit,
		CommitHashes: []string{"missing"},
		Branch:       "main",
	})

	q := mustQuery(t, db, time.UTC)
	got, err := q.WeekWork(ISOWeek{Year: 2026, Week: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("WeekWork() len = %d, want 3: %#v", len(got), got)
	}
	if got[0].Day != (Date{Year: 2025, Month: time.December, Day: 29}) || got[0].Slug != "hackatrak" || got[0].Source != SourceFile {
		t.Fatalf("first = %+v, want 2025-12-29 hackatrak file", got[0])
	}
	if got[1].Slug != "hackatrak" || got[1].Day.Day != 4 || got[1].FileSample != "2026-01-04.go" {
		t.Fatalf("second = %+v", got[1])
	}
	if got[2].Slug != "staksmith" || got[2].Source != SourceCommit || !reflect.DeepEqual(got[2].Commits, []Commit{{Hash: "missing"}}) {
		t.Fatalf("third = %#v, want hash-only commit", got[2])
	}
	if got[2].Ticket != "" || got[0].Worked != true {
		t.Fatalf("WeekWork() = %#v", got)
	}
}

func TestWeekWorkRejectsInvalid(t *testing.T) {
	q := mustQuery(t, openQueryDB(t), time.UTC)
	for _, iso := range []ISOWeek{{Year: 2026, Week: 0}, {Year: 2026, Week: 54}, {Year: 2025, Week: 53}, {}} {
		if _, err := q.WeekWork(iso); err == nil {
			t.Fatalf("WeekWork(%+v) error = nil, want an error", iso)
		}
	}
}

func TestWorkRequiresQuery(t *testing.T) {
	var q *Query
	if _, err := q.TodaysWork(); err == nil {
		t.Fatal("nil TodaysWork error = nil, want an error")
	}
	if _, err := q.WeekWork(ISOWeek{Year: 2026, Week: 1}); err == nil {
		t.Fatal("nil WeekWork error = nil, want an error")
	}
}

func mustQuery(t *testing.T, db *store.DB, loc *time.Location) *Query {
	t.Helper()
	q, err := New(db, loc)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func seedPresence(t *testing.T, db *store.DB, row store.DayPresence) {
	t.Helper()
	if _, err := db.UpsertDayPresence(row); err != nil {
		t.Fatal(err)
	}
}

func seedCommit(t *testing.T, db *store.DB, hash, branch, subject, body string) {
	t.Helper()
	_, err := db.InsertEvent(store.Event{
		Kind:        store.KindCommit,
		Hash:        hash,
		RepoPath:    "/repo/" + hash,
		ProjectSlug: "staksmith",
		OccurredAt:  time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC),
		Branch:      branch,
		Subject:     subject,
		Body:        body,
	})
	if err != nil && !errors.Is(err, store.ErrDuplicate) {
		t.Fatal(err)
	}
}
