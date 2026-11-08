package derive

import (
	"reflect"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestFromEventsGroupsByProjectAndLocalDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	// 02:00 UTC on the 6th is still the 5th in Chicago. UTC grouping would be wrong.
	late := time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)
	next := time.Date(2026, 11, 6, 15, 0, 0, 0, loc)

	events := []store.Event{
		{
			Kind:        store.KindCommit,
			Hash:        "bbb",
			ProjectSlug: "staksmith",
			OccurredAt:  late.Add(time.Minute),
			Branch:      "feat/p4",
			Subject:     "second",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "aaa",
			ProjectSlug: "staksmith",
			OccurredAt:  late,
			Branch:      "main",
			Subject:     "first",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "aaa",
			ProjectSlug: "staksmith",
			OccurredAt:  late.Add(2 * time.Minute),
			Branch:      "ignored",
		},
		{
			Kind:        store.KindFileTouch,
			ProjectSlug: "staksmith",
			OccurredAt:  late.Add(3 * time.Minute),
			FileCount:   2,
			FileSample:  "a.go, b.go",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "ccc",
			ProjectSlug: "staksmith",
			OccurredAt:  next,
			Branch:      "main",
		},
		{
			Kind:        store.KindCommit,
			Hash:        "ddd",
			ProjectSlug: "hackatrak",
			OccurredAt:  late,
			Branch:      "main",
		},
	}

	got, err := FromEvents(events, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := []DayPresence{
		{
			ProjectSlug:  "hackatrak",
			Day:          "2026-11-05",
			Worked:       true,
			Source:       SourceCommit,
			Branch:       "main",
			CommitHashes: []string{"ddd"},
		},
		{
			ProjectSlug:  "staksmith",
			Day:          "2026-11-05",
			Worked:       true,
			Source:       SourceCommit,
			Branch:       "feat/p4",
			CommitHashes: []string{"aaa", "bbb"},
			FileCount:    2,
			FileSample:   "a.go, b.go",
		},
		{
			ProjectSlug:  "staksmith",
			Day:          "2026-11-06",
			Worked:       true,
			Source:       SourceCommit,
			Branch:       "main",
			CommitHashes: []string{"ccc"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromEvents() = %#v, want %#v", got, want)
	}
}

func TestFromEventsFileOnly(t *testing.T) {
	loc := time.FixedZone("CST", -6*60*60)
	when := time.Date(2026, 11, 5, 18, 30, 0, 0, loc)
	events := []store.Event{
		{
			Kind:        store.KindFileTouch,
			ProjectSlug: "notes",
			OccurredAt:  when.Add(-time.Hour),
			FileCount:   1,
			FileSample:  "old.md",
		},
		{
			Kind:        store.KindFileTouch,
			ProjectSlug: "notes",
			OccurredAt:  when,
			FileCount:   4,
			FileSample:  "today.md",
		},
	}

	got, err := FromEvents(events, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := []DayPresence{{
		ProjectSlug: "notes",
		Day:         "2026-11-05",
		Worked:      true,
		Source:      SourceFile,
		FileCount:   5,
		FileSample:  "today.md",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FromEvents() = %#v, want %#v", got, want)
	}
}

func TestFromEventsSkipsUnassignedAndLeavesInput(t *testing.T) {
	loc := time.UTC
	events := []store.Event{
		{Kind: store.KindCommit, Hash: "zzz", ProjectSlug: "  ", OccurredAt: time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)},
		{Kind: store.KindFileTouch, ProjectSlug: "", OccurredAt: time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC), FileCount: 1},
		{Kind: "command", ProjectSlug: "staksmith", OccurredAt: time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC)},
	}
	before := append([]store.Event(nil), events...)

	got, err := FromEvents(events, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("FromEvents() = %#v, want nil", got)
	}
	if !reflect.DeepEqual(events, before) {
		t.Fatalf("input mutated: %#v", events)
	}
}

func TestFromEventsNilLocationUsesLocalDay(t *testing.T) {
	when := time.Date(2026, 11, 5, 12, 0, 0, 0, time.Local)
	got, err := FromEvents([]store.Event{{
		Kind:        store.KindCommit,
		Hash:        "abc",
		ProjectSlug: "hackatrak",
		OccurredAt:  when,
		Branch:      "main",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantDay := when.In(time.Local).Format("2006-01-02")
	if len(got) != 1 || got[0].Day != wantDay || got[0].Source != SourceCommit || !got[0].Worked {
		t.Fatalf("FromEvents() = %#v, want local day %s", got, wantDay)
	}
}

func TestFromEventsRejectsMissingTime(t *testing.T) {
	_, err := FromEvents([]store.Event{{
		Kind:        store.KindCommit,
		Hash:        "abc",
		ProjectSlug: "hackatrak",
	}}, time.UTC)
	if err == nil {
		t.Fatal("FromEvents() error = nil, want missing occurred_at")
	}
}
