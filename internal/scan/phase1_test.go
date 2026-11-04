package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// TestPhase1 is the Phase 1 acceptance gate (P1.T): parse sample git log
// output, a re-run is idempotent because the cursor is respected, and the
// author filter excludes other people's commits.
func TestPhase1(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")
	git(t, repo, "checkout", "-b", "feature/p1")

	loc := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, 11, 3, 18, 0, 0, 0, loc)
	since := LocalMidnight(now).Format(time.RFC3339)
	authors := []string{"me@example.com"}

	gitCommit(t, repo, now.Add(-24*time.Hour).Format(time.RFC3339), "me@example.com", "Me", "yesterday", "")
	gitCommit(t, repo, time.Date(2026, 11, 3, 9, 30, 0, 0, loc).Format(time.RFC3339), "me@example.com", "Me", "ship parser", "line one\nline two")
	gitCommit(t, repo, time.Date(2026, 11, 3, 10, 0, 0, 0, loc).Format(time.RFC3339), "teammate@example.com", "Them", "not mine", "")
	gitCommit(t, repo, time.Date(2026, 11, 3, 11, 0, 0, 0, loc).Format(time.RFC3339), "me@example.com", "Me", "second today", "")

	t.Run("parse sample git log", func(t *testing.T) {
		raw := sampleGitLog(t, repo, since, authors)
		got, err := parseCommitLog(raw)
		if err != nil {
			t.Fatalf("parseCommitLog() error = %v\nraw:\n%s", err, raw)
		}
		if subjects(got) != "ship parser,second today" {
			t.Fatalf("parsed subjects = %q, want only my commits since midnight", subjects(got))
		}
		if got[0].Branch != "feature/p1" || got[0].Body != "line one\nline two" || len(got[0].Hash) != 40 {
			t.Fatalf("first parsed commit = %#v", got[0])
		}
		wantTime := time.Date(2026, 11, 3, 9, 30, 0, 0, loc)
		if !got[0].Time.Equal(wantTime) || got[0].Time.Format(time.RFC3339) != "2026-11-03T09:30:00-06:00" {
			t.Fatalf("parsed time = %s, want %s", got[0].Time.Format(time.RFC3339), wantTime.Format(time.RFC3339))
		}
	})

	db := openScanDB(t)
	first, err := CollectGit(db, []string{root}, authors, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if subjects(detected(first)) != "ship parser,second today" {
		t.Fatalf("first collect = %q, want my commits only", subjects(detected(first)))
	}
	if first.Inserted != 2 || first.Skipped != 0 {
		t.Fatalf("first collect counts = inserted %d skipped %d, want 2 inserted", first.Inserted, first.Skipped)
	}
	repoPath := first.Commits[0].RepoPath
	assertEventSubjects(t, db, repoPath, "ship parser", "second today")

	state, err := db.SyncState(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Cursor != now.Format(time.RFC3339) {
		t.Fatalf("cursor = %q, want %q", state.Cursor, now.Format(time.RFC3339))
	}

	secondNow := now.Add(time.Hour)
	second, err := CollectGit(db, []string{root}, authors, secondNow, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Commits) != 0 || second.Inserted != 0 {
		t.Fatalf("re-run = %+v, want cursor to hide already scanned commits", second)
	}
	assertEventSubjects(t, db, repoPath, "ship parser", "second today")

	later := secondNow.Add(30 * time.Minute)
	gitCommit(t, repo, later.Format(time.RFC3339), "me@example.com", "Me", "after cursor", "")
	gitCommit(t, repo, later.Add(time.Minute).Format(time.RFC3339), "teammate@example.com", "Them", "still not mine", "")

	third, err := CollectGit(db, []string{root}, authors, later.Add(time.Minute), false)
	if err != nil {
		t.Fatal(err)
	}
	if subjects(detected(third)) != "after cursor" || third.Inserted != 1 {
		t.Fatalf("post-cursor collect = %+v, want only after cursor", third)
	}
	assertEventSubjects(t, db, repoPath, "ship parser", "second today", "after cursor")
}

func sampleGitLog(t *testing.T, repo, since string, authors []string) string {
	t.Helper()
	emails, err := authorEmails(authors)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"-C", repo, "--no-pager", "log", "--reverse", "--branches", "--source",
		"--since", since, "--pretty=format:" + commitLogFormat,
	}
	for _, email := range emails {
		args = append(args, "--author="+email)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("sample git log: %v", err)
	}
	return string(out)
}

func detected(collected GitCollect) []Commit {
	commits := make([]Commit, len(collected.Commits))
	for i, commit := range collected.Commits {
		commits[i] = commit.Commit
	}
	return commits
}

func assertEventSubjects(t *testing.T, db *store.DB, repo string, want ...string) {
	t.Helper()
	events, err := db.EventsByRepo(repo)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(events))
	for i, event := range events {
		got[i] = event.Subject
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %q, want %q", strings.Join(got, ","), strings.Join(want, ","))
	}
}
