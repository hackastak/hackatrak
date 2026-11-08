package query

import (
	"reflect"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/derive"
	"github.com/hackastak/hackatrak/internal/store"
)

// TestPhase4 is the Phase 4 acceptance gate (P4.T): seeded events derive into
// the stored project-day rows, and the query seam returns those days as typed
// structs. A file-only day still counts as worked, with source file.
func TestPhase4(t *testing.T) {
	loc := time.FixedZone("CST", -6*60*60)
	day := time.Date(2026, 11, 5, 15, 0, 0, 0, loc)
	next := time.Date(2026, 11, 6, 10, 0, 0, 0, loc)

	db := openQueryDB(t)
	seedPhase4(t, db, []store.Event{
		{
			Kind:        store.KindCommit,
			Hash:        "bbb",
			RepoPath:    "/Developer/staksmith",
			ProjectSlug: "staksmith",
			OccurredAt:  day.Add(time.Hour),
			Branch:      "feat/p4",
			Subject:     "second",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "aaa",
			RepoPath:    "/Developer/staksmith",
			ProjectSlug: "staksmith",
			OccurredAt:  day,
			Branch:      "main",
			Subject:     "first",
			Body:        "detail",
		},
		{
			Kind:        store.KindFileTouch,
			RepoPath:    "/Developer/staksmith",
			ProjectSlug: "staksmith",
			OccurredAt:  day.Add(90 * time.Minute),
			FileCount:   2,
			FileSample:  "a.go, b.go",
		},
		{
			Kind:        store.KindFileTouch,
			RepoPath:    "/Developer/notes",
			ProjectSlug: "notes",
			OccurredAt:  day.Add(2 * time.Hour),
			FileCount:   4,
			FileSample:  "today.md",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "ccc",
			RepoPath:    "/Developer/hackatrak",
			ProjectSlug: "hackatrak",
			OccurredAt:  next,
			Branch:      "main",
			Subject:     "ship query",
		},
	})

	events := loadPhase4Events(t, db, []string{
		"/Developer/hackatrak",
		"/Developer/notes",
		"/Developer/staksmith",
	})
	derived, err := derive.FromEvents(events, loc)
	if err != nil {
		t.Fatal(err)
	}
	persistDerived(t, db, derived)

	t.Run("derived rows", func(t *testing.T) {
		got := loadPhase4Presence(t, db, []presenceKey{
			{"hackatrak", "2026-11-06"},
			{"notes", "2026-11-05"},
			{"staksmith", "2026-11-05"},
		})
		want := []store.DayPresence{
			{
				ProjectSlug:  "hackatrak",
				Day:          "2026-11-06",
				Worked:       true,
				Source:       store.PresenceCommit,
				Branch:       "main",
				CommitHashes: []string{"ccc"},
			},
			{
				ProjectSlug: "notes",
				Day:         "2026-11-05",
				Worked:      true,
				Source:      store.PresenceFile,
				FileCount:   4,
				FileSample:  "today.md",
			},
			{
				ProjectSlug:  "staksmith",
				Day:          "2026-11-05",
				Worked:       true,
				Source:       store.PresenceCommit,
				Branch:       "feat/p4",
				CommitHashes: []string{"aaa", "bbb"},
				FileCount:    2,
				FileSample:   "a.go, b.go",
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("derived rows = %#v, want %#v", got, want)
		}
	})

	t.Run("file-only day", func(t *testing.T) {
		row, err := db.DayPresence("notes", "2026-11-05")
		if err != nil {
			t.Fatal(err)
		}
		if !row.Worked || row.Source != store.PresenceFile || len(row.CommitHashes) != 0 {
			t.Fatalf("file-only row = %+v, want worked=true source=file", row)
		}
	})

	q := mustQuery(t, db, loc)
	q.now = func() time.Time { return day }

	t.Run("today", func(t *testing.T) {
		got, err := q.TodaysWork()
		if err != nil {
			t.Fatal(err)
		}
		want := []ProjectDay{
			{
				Slug:       "notes",
				Day:        Date{Year: 2026, Month: time.November, Day: 5},
				Worked:     true,
				Source:     SourceFile,
				FileCount:  4,
				FileSample: "today.md",
			},
			{
				Slug:   "staksmith",
				Day:    Date{Year: 2026, Month: time.November, Day: 5},
				Worked: true,
				Source: SourceCommit,
				Branch: "feat/p4",
				Commits: []Commit{
					{Hash: "aaa", Branch: "main", Subject: "first", Body: "detail"},
					{Hash: "bbb", Branch: "feat/p4", Subject: "second"},
				},
				FileCount:  2,
				FileSample: "a.go, b.go",
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("TodaysWork() = %#v, want %#v", got, want)
		}
	})

	t.Run("week", func(t *testing.T) {
		year, week := day.ISOWeek()
		got, err := q.WeekWork(ISOWeek{Year: year, Week: week})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("WeekWork() len = %d, want 3: %#v", len(got), got)
		}
		if got[0].Slug != "notes" || !got[0].Worked || got[0].Source != SourceFile {
			t.Fatalf("week[0] = %+v, want file-only notes", got[0])
		}
		if got[1].Slug != "staksmith" || got[1].Source != SourceCommit || len(got[1].Commits) != 2 {
			t.Fatalf("week[1] = %#v, want staksmith commits", got[1])
		}
		if got[2].Slug != "hackatrak" || got[2].Day != (Date{Year: 2026, Month: time.November, Day: 6}) {
			t.Fatalf("week[2] = %+v, want hackatrak on the 6th", got[2])
		}
		if got[2].Commits[0].Subject != "ship query" {
			t.Fatalf("week[2] commits = %#v", got[2].Commits)
		}
	})
}

type presenceKey struct {
	slug string
	day  string
}

func seedPhase4(t *testing.T, db *store.DB, events []store.Event) {
	t.Helper()
	for _, event := range events {
		if _, err := db.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}
}

func loadPhase4Events(t *testing.T, db *store.DB, repos []string) []store.Event {
	t.Helper()
	var events []store.Event
	for _, repo := range repos {
		found, err := db.EventsByRepo(repo)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, found...)
	}
	if len(events) == 0 {
		t.Fatal("seeded events = 0")
	}
	return events
}

func persistDerived(t *testing.T, db *store.DB, rows []derive.DayPresence) {
	t.Helper()
	for _, row := range rows {
		if _, err := db.UpsertDayPresence(store.DayPresence{
			ProjectSlug:  row.ProjectSlug,
			Day:          row.Day,
			Worked:       row.Worked,
			Source:       store.PresenceSource(row.Source),
			Branch:       row.Branch,
			CommitHashes: row.CommitHashes,
			FileCount:    row.FileCount,
			FileSample:   row.FileSample,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func loadPhase4Presence(t *testing.T, db *store.DB, keys []presenceKey) []store.DayPresence {
	t.Helper()
	rows := make([]store.DayPresence, 0, len(keys))
	for _, key := range keys {
		row, err := db.DayPresence(key.slug, key.day)
		if err != nil {
			t.Fatal(err)
		}
		row.ID = 0
		rows = append(rows, row)
	}
	return rows
}
