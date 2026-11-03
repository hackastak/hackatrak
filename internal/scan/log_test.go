package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCommitLog(t *testing.T) {
	raw := "" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\x1f2026-11-02T09:30:00-06:00\x1ffeature/p1.3\x1fship parser\x1fline one\nline two\n\x1e\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\x1f2026-11-02T11:00:00-06:00\x1frefs/heads/main\x1fempty body\x1f\x1e\n"

	got, err := parseCommitLog(raw)
	if err != nil {
		t.Fatalf("parseCommitLog() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("parseCommitLog() len = %d, want 2", len(got))
	}

	if got[0].Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		got[0].Branch != "feature/p1.3" ||
		got[0].Subject != "ship parser" ||
		got[0].Body != "line one\nline two" {
		t.Fatalf("first commit = %#v", got[0])
	}
	wantTime := time.Date(2026, 11, 2, 9, 30, 0, 0, time.FixedZone("", -6*3600))
	if !got[0].Time.Equal(wantTime) {
		t.Fatalf("first time = %s, want %s", got[0].Time, wantTime)
	}
	if got[0].Time.Format(time.RFC3339) != "2026-11-02T09:30:00-06:00" {
		t.Fatalf("first time format = %s, want offset preserved", got[0].Time.Format(time.RFC3339))
	}
	if got[1].Branch != "main" || got[1].Body != "" || got[1].Subject != "empty body" {
		t.Fatalf("second commit = %#v", got[1])
	}
}

func TestParseCommitLogEmptyAndMalformed(t *testing.T) {
	got, err := parseCommitLog("")
	if err != nil {
		t.Fatalf("empty log error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty log = %#v, want none", got)
	}

	if _, err := parseCommitLog("not-a-record\x1e"); err == nil {
		t.Fatal("malformed log error = nil, want failure")
	}
}

func TestLocalMidnight(t *testing.T) {
	loc := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, 11, 2, 23, 26, 0, 0, loc)
	got := LocalMidnight(now)
	want := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	if !got.Equal(want) || got.Location().String() != loc.String() {
		t.Fatalf("LocalMidnight() = %s, want %s", got, want)
	}
}

func TestCommitsSince(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	gitCommit(t, repo, "2026-11-01T12:00:00-06:00", "me@example.com", "Me", "yesterday", "")
	git(t, repo, "checkout", "-b", "feature/p1.3")
	gitCommit(t, repo, "2026-11-02T09:30:00-06:00", "me@example.com", "Me", "ship parser", "line one\nline two")
	gitCommit(t, repo, "2026-11-02T10:00:00-06:00", "also@example.com", "Also", "second account", "")
	gitCommit(t, repo, "2026-11-02T10:30:00-06:00", "teammate@example.com", "Them", "not mine", "")
	gitCommit(t, repo, "2026-11-02T11:00:00-06:00", "meXexample.com", "Dot", "regex trap", "")

	got, err := CommitsSince(repo, "2026-11-02T00:00:00-06:00", []string{" me@example.com ", "also@example.com"})
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("CommitsSince() len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Subject != "ship parser" || got[0].Branch != "feature/p1.3" || got[0].Body != "line one\nline two" {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].Subject != "second account" || got[1].Branch != "feature/p1.3" {
		t.Fatalf("second = %#v", got[1])
	}
	if !got[0].Time.Before(got[1].Time) {
		t.Fatalf("commits not oldest-first: %#v", got)
	}
	for _, c := range got {
		if len(c.Hash) != 40 {
			t.Fatalf("hash = %q, want 40 hex chars", c.Hash)
		}
	}
}

func TestCommitsSinceDefaultsToLocalMidnight(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")

	now := time.Now()
	start := LocalMidnight(now)
	today := start.Add(time.Minute)
	if today.After(now) {
		today = now
	}
	gitCommit(t, repo, start.Add(-time.Hour).Format(time.RFC3339), "me@example.com", "Me", "before midnight", "")
	gitCommit(t, repo, today.Format(time.RFC3339), "me@example.com", "Me", "today", "")

	got, err := CommitsSince(repo, "", []string{"me@example.com"})
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(got) != 1 || got[0].Subject != "today" {
		t.Fatalf("CommitsSince() = %#v, want only today's commit", got)
	}
}

func TestCommitsSinceRejectsEmptyAuthors(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")

	for _, authors := range [][]string{nil, {}, {"", "  "}} {
		_, err := CommitsSince(repo, "2026-11-02T00:00:00-06:00", authors)
		if err == nil {
			t.Fatalf("CommitsSince(%q) error = nil, want missing authors", authors)
		}
	}
}

func TestCommitsSinceRejectsNonRepo(t *testing.T) {
	_, err := CommitsSince(t.TempDir(), "2026-11-02T00:00:00-06:00", []string{"me@example.com"})
	if err == nil {
		t.Fatal("CommitsSince() error = nil, want non-repo failure")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("CommitsSince() error = %v, want not a git repository", err)
	}
}

func gitCommit(t *testing.T, dir, when, email, name, subject, body string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required for log tests")
	}
	args := []string{
		"-c", "user.email=" + email,
		"-c", "user.name=" + name,
		"commit", "--allow-empty",
		"-m", subject,
	}
	if body != "" {
		args = append(args, "-m", body)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gitconfig"),
		"GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_DATE="+when,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit %q: %v\n%s", subject, err, out)
	}
}
