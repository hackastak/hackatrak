package scan

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestPersistCommitsInsertsEvents(t *testing.T) {
	db := openScanDB(t)
	when := time.Date(2026, 11, 3, 9, 30, 0, 0, time.FixedZone("CST", -6*3600))
	repo := "/Users/hackastak/Developer/Hackatrak"

	got, err := PersistCommits(db, repo, "Hackatrak", []Commit{{
		Hash:    "abc123",
		Time:    when,
		Branch:  "main",
		Subject: "feat: persist commits",
		Body:    "dedupe by hash",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 0 || len(got.Inserted) != 1 {
		t.Fatalf("PersistCommits = %+v, want 1 inserted, 0 skipped", got)
	}

	event := got.Inserted[0]
	if event.ID == 0 || event.Kind != store.KindCommit || event.Hash != "abc123" {
		t.Fatalf("inserted = %+v", event)
	}
	if event.RepoPath != repo || event.ProjectSlug != "Hackatrak" {
		t.Fatalf("inserted path/slug = %+v", event)
	}
	if event.Branch != "main" || event.Subject != "feat: persist commits" || event.Body != "dedupe by hash" {
		t.Fatalf("inserted fields = %+v", event)
	}
	if !event.OccurredAt.Equal(when) {
		t.Fatalf("OccurredAt = %s, want %s", event.OccurredAt, when)
	}

	stored, err := db.EventByHash("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != event.ID || stored.Kind != store.KindCommit {
		t.Fatalf("EventByHash = %+v", stored)
	}
}

func TestPersistCommitsDedupesByHash(t *testing.T) {
	db := openScanDB(t)
	repo := "/Users/hackastak/Developer/Hackatrak"
	commits := []Commit{
		{Hash: "aaa", Time: time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC), Subject: "first"},
		{Hash: "bbb", Time: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), Subject: "second"},
	}

	first, err := PersistCommits(db, repo, "", commits)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Inserted) != 2 || first.Skipped != 0 {
		t.Fatalf("first persist = %+v", first)
	}

	again, err := PersistCommits(db, repo, "", commits)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Inserted) != 0 || again.Skipped != 2 {
		t.Fatalf("second persist = %+v, want 0 inserted and 2 skipped", again)
	}

	stored, err := db.EventsByRepo(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored len = %d, want 2", len(stored))
	}
}

func TestPersistCommitsSkipsKnownHashInMixedBatch(t *testing.T) {
	db := openScanDB(t)
	repo := "/repo"
	when := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	if _, err := PersistCommits(db, repo, "slug", []Commit{{
		Hash: "known", Time: when, Subject: "already there",
	}}); err != nil {
		t.Fatal(err)
	}

	got, err := PersistCommits(db, repo, "slug", []Commit{
		{Hash: "known", Time: when, Subject: "already there"},
		{Hash: "new", Time: when.Add(time.Hour), Subject: "fresh", Branch: "feature"},
		{Hash: "new", Time: when.Add(time.Hour), Subject: "fresh again"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 2 || len(got.Inserted) != 1 || got.Inserted[0].Hash != "new" {
		t.Fatalf("mixed persist = %+v, want new inserted and known+repeat skipped", got)
	}
	if got.Inserted[0].Subject != "fresh" {
		t.Fatalf("kept subject = %q, want first occurrence", got.Inserted[0].Subject)
	}

	stored, err := db.EventsByRepo(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored len = %d, want 2", len(stored))
	}
}

func TestPersistCommitsDedupesAcrossRepos(t *testing.T) {
	db := openScanDB(t)
	when := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	commit := Commit{Hash: "same", Time: when, Subject: "shared object"}

	if _, err := PersistCommits(db, "/repo-a", "a", []Commit{commit}); err != nil {
		t.Fatal(err)
	}
	got, err := PersistCommits(db, "/repo-b", "b", []Commit{commit})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inserted) != 0 || got.Skipped != 1 {
		t.Fatalf("cross-repo persist = %+v, want hash deduped", got)
	}
}

func TestPersistCommitsEmptyInput(t *testing.T) {
	db := openScanDB(t)
	got, err := PersistCommits(db, "/repo", "slug", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Inserted) != 0 || got.Skipped != 0 {
		t.Fatalf("empty persist = %+v", got)
	}
}

func TestPersistCommitsRejectsIncomplete(t *testing.T) {
	db := openScanDB(t)
	when := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	valid := Commit{Hash: "ok", Time: when, Subject: "keep me out"}

	cases := []struct {
		name   string
		repo   string
		commits []Commit
	}{
		{name: "empty repo", repo: "", commits: []Commit{valid}},
		{name: "empty hash", repo: "/repo", commits: []Commit{{Time: when}}},
		{name: "zero time", repo: "/repo", commits: []Commit{{Hash: "x"}}},
		{name: "nil db later", repo: "/repo", commits: []Commit{valid}},
	}
	for _, tc := range cases {
		if tc.name == "nil db later" {
			continue
		}
		if _, err := PersistCommits(db, tc.repo, "", tc.commits); err == nil {
			t.Fatalf("%s: error = nil, want an error", tc.name)
		}
	}
	if _, err := PersistCommits(nil, "/repo", "", []Commit{valid}); err == nil {
		t.Fatal("nil db: error = nil, want an error")
	}

	stored, err := db.EventsByRepo("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("invalid persist wrote %d events, want 0", len(stored))
	}
}

func TestPersistCommitsDoesNotMutateInput(t *testing.T) {
	db := openScanDB(t)
	commits := []Commit{{
		Hash: "abc", Time: time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC), Subject: "orig",
	}}
	before := commits[0]
	if _, err := PersistCommits(db, "/repo", "", commits); err != nil {
		t.Fatal(err)
	}
	if commits[0] != before {
		t.Fatalf("input mutated: got %+v, want %+v", commits[0], before)
	}
}

func TestPersistCommitsSurfacesStoreErrors(t *testing.T) {
	db := openScanDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := PersistCommits(db, "/repo", "", []Commit{{
		Hash: "abc", Time: time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC),
	}})
	if err == nil {
		t.Fatal("closed db: error = nil, want an error")
	}
	if errors.Is(err, store.ErrDuplicate) {
		t.Fatal("closed db reported a duplicate, want a store error")
	}
}

func openScanDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "hackatrak.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
