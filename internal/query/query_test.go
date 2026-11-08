package query

import (
	"fmt"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestProjectDayIsData(t *testing.T) {
	day := ProjectDay{
		Slug:   "staksmith",
		Name:   "staksmith",
		Root:   "/Users/hackastak/Developer/staksmith",
		Bucket: "Hackastak",
		Day:    Date{Year: 2026, Month: time.November, Day: 6},
		Worked: true,
		Source: SourceCommit,
		Branch: "main",
		Ticket: "NEXTGEN-1",
		Commits: []Commit{{
			Hash:    "aaa",
			Branch:  "main",
			Subject: "rework docs",
			Body:    "detail",
		}},
		FileCount:  2,
		FileSample: "a.go, b.go",
	}
	if day.Slug != "staksmith" || day.Bucket != "Hackastak" || !day.Worked {
		t.Fatalf("ProjectDay = %+v", day)
	}
	if day.Day != (Date{Year: 2026, Month: time.November, Day: 6}) {
		t.Fatalf("Day = %+v", day.Day)
	}
	if day.Source != SourceCommit || day.Ticket != "NEXTGEN-1" || day.FileCount != 2 {
		t.Fatalf("ProjectDay = %+v", day)
	}
	if len(day.Commits) != 1 || day.Commits[0].Hash != "aaa" || day.Commits[0].Subject != "rework docs" {
		t.Fatalf("Commits = %+v", day.Commits)
	}
	assertNotStringer(t, day)
	assertNotStringer(t, day.Day)
	assertNotStringer(t, day.Commits[0])
	assertNotStringer(t, ISOWeek{Year: 2026, Week: 45})
}

func TestSourceValuesMatchStoredPresence(t *testing.T) {
	if SourceCommit != "commit" || SourceFile != "file" {
		t.Fatalf("SourceCommit=%q SourceFile=%q, want commit and file", SourceCommit, SourceFile)
	}
	if string(SourceCommit) != string(store.PresenceCommit) || string(SourceFile) != string(store.PresenceFile) {
		t.Fatal("query source values drifted from store presence sources")
	}
}

func TestZeroProjectDay(t *testing.T) {
	var day ProjectDay
	if day.Worked || day.Source != "" || day.Commits != nil || day.FileCount != 0 {
		t.Fatalf("zero ProjectDay = %+v", day)
	}
}

func TestNewRejectsNilDB(t *testing.T) {
	if _, err := New(nil, time.UTC); err == nil {
		t.Fatal("New(nil) error = nil, want an error")
	}
}

func TestNewNilLocationUsesLocal(t *testing.T) {
	db := openQueryDB(t)
	q, err := New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.db != db || q.loc != time.Local {
		t.Fatalf("New location = %v, want local", q.loc)
	}
	assertNotStringer(t, q)
}

func assertNotStringer(t *testing.T, v any) {
	t.Helper()
	if _, ok := v.(fmt.Stringer); ok {
		t.Fatalf("%T formats itself; query types are data only", v)
	}
}

func openQueryDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/hackatrak.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
