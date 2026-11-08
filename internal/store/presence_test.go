package store

import (
	"errors"
	"reflect"
	"testing"
)

func TestUpsertDayPresenceInsertsAndReplaces(t *testing.T) {
	db := openTestDB(t)

	_, err := db.DayPresence("staksmith", "2026-11-06")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("DayPresence error = %v, want ErrNotFound", err)
	}

	inserted, err := db.UpsertDayPresence(DayPresence{
		ProjectSlug:  "staksmith",
		Day:          "2026-11-06",
		Worked:       true,
		Source:       PresenceCommit,
		Branch:       "main",
		Ticket:       "NEXTGEN-1",
		CommitHashes: []string{"aaa", "bbb"},
		FileCount:    2,
		FileSample:   "a.go, b.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted.ID == 0 {
		t.Fatal("UpsertDayPresence ID = 0, want a row id")
	}

	got, err := db.DayPresence("staksmith", "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != inserted.ID || got.Source != PresenceCommit || got.Branch != "main" || got.Ticket != "NEXTGEN-1" {
		t.Fatalf("DayPresence = %+v", got)
	}
	if !got.Worked || got.FileCount != 2 || got.FileSample != "a.go, b.go" {
		t.Fatalf("DayPresence = %+v", got)
	}
	if !reflect.DeepEqual(got.CommitHashes, []string{"aaa", "bbb"}) {
		t.Fatalf("CommitHashes = %#v", got.CommitHashes)
	}

	replaced, err := db.UpsertDayPresence(DayPresence{
		ProjectSlug: "staksmith",
		Day:         "2026-11-06",
		Worked:      true,
		Source:      PresenceFile,
		FileCount:   4,
		FileSample:  "today.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ID != inserted.ID {
		t.Fatalf("replaced ID = %d, want %d", replaced.ID, inserted.ID)
	}
	if replaced.Source != PresenceFile || replaced.Branch != "" || replaced.Ticket != "" || replaced.CommitHashes != nil {
		t.Fatalf("replaced = %+v, want file presence with cleared commit fields", replaced)
	}
	if replaced.FileCount != 4 || replaced.FileSample != "today.md" || !replaced.Worked {
		t.Fatalf("replaced = %+v", replaced)
	}

	other, err := db.UpsertDayPresence(DayPresence{
		ProjectSlug:  "hackatrak",
		Day:          "2026-11-06",
		Worked:       true,
		Source:       PresenceCommit,
		CommitHashes: []string{"ccc"},
		Branch:       "feat/p4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == inserted.ID {
		t.Fatal("other project reused the staksmith row id")
	}
	still, err := db.DayPresence("staksmith", "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if still.Source != PresenceFile || still.FileSample != "today.md" {
		t.Fatalf("staksmith row changed by other upsert: %+v", still)
	}
}

func TestUpsertDayPresenceRejectsInvalid(t *testing.T) {
	db := openTestDB(t)
	valid := DayPresence{
		ProjectSlug:  "staksmith",
		Day:          "2026-11-06",
		Worked:       true,
		Source:       PresenceCommit,
		CommitHashes: []string{"aaa"},
	}
	cases := []DayPresence{
		{},
		{ProjectSlug: "  ", Day: "2026-11-06", Worked: true, Source: PresenceFile},
		{ProjectSlug: "staksmith", Day: "11/06/2026", Worked: true, Source: PresenceFile},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: "time"},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceCommit},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceFile, CommitHashes: []string{"aaa"}},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceCommit, CommitHashes: []string{"aaa", "aaa"}},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceCommit, CommitHashes: []string{"bad\nhash"}},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceFile, FileCount: -1},
	}
	for _, row := range cases {
		if _, err := db.UpsertDayPresence(row); err == nil {
			t.Fatalf("UpsertDayPresence(%+v) error = nil, want an error", row)
		}
	}
	if _, err := db.UpsertDayPresence(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DayPresence("", "2026-11-06"); err == nil {
		t.Fatal("DayPresence(empty slug) error = nil, want an error")
	}
}

func TestUpsertDayPresenceLeavesHashOrder(t *testing.T) {
	db := openTestDB(t)
	hashes := []string{"bbb", "aaa"}
	if _, err := db.UpsertDayPresence(DayPresence{
		ProjectSlug:  "staksmith",
		Day:          "2026-11-05",
		Worked:       true,
		Source:       PresenceCommit,
		CommitHashes: hashes,
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hashes, []string{"bbb", "aaa"}) {
		t.Fatalf("input hashes mutated: %#v", hashes)
	}
	got, err := db.DayPresence("staksmith", "2026-11-05")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.CommitHashes, []string{"bbb", "aaa"}) {
		t.Fatalf("CommitHashes = %#v, want caller order", got.CommitHashes)
	}
}

func TestDayPresencesInclusiveRange(t *testing.T) {
	db := openTestDB(t)
	for _, row := range []DayPresence{
		{ProjectSlug: "staksmith", Day: "2026-11-05", Worked: true, Source: PresenceFile, FileCount: 1},
		{ProjectSlug: "hackatrak", Day: "2026-11-06", Worked: true, Source: PresenceCommit, CommitHashes: []string{"aaa"}},
		{ProjectSlug: "staksmith", Day: "2026-11-06", Worked: true, Source: PresenceFile, FileCount: 2},
		{ProjectSlug: "staksmith", Day: "2026-11-07", Worked: true, Source: PresenceFile, FileCount: 3},
	} {
		if _, err := db.UpsertDayPresence(row); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.DayPresences("2026-11-06", "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProjectSlug != "hackatrak" || got[1].ProjectSlug != "staksmith" {
		t.Fatalf("DayPresences one day = %+v, want hackatrak then staksmith", got)
	}

	span, err := db.DayPresences("2026-11-05", "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(span) != 3 || span[0].Day != "2026-11-05" || span[2].Day != "2026-11-06" {
		t.Fatalf("DayPresences span = %+v", span)
	}

	empty, err := db.DayPresences("2026-11-08", "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if empty != nil {
		t.Fatalf("empty range = %+v, want nil", empty)
	}
	if _, err := db.DayPresences("2026-11-07", "2026-11-06"); err == nil {
		t.Fatal("from after to error = nil, want an error")
	}
	if _, err := db.DayPresences("", "2026-11-06"); err == nil {
		t.Fatal("empty from error = nil, want an error")
	}
}
