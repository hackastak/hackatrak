package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

func TestSyncDryRunPrintsCommitsWithoutWriting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, commitOnToday(-2*time.Minute), "me@example.com", "Me", "wire the scan")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "other@example.com", "Other", "teammate work")
	writeSyncConfig(t, home, root)

	out, err := execute(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "wire the scan") {
		t.Fatalf("output = %q, want detected commit", out)
	}
	if strings.Contains(out, "teammate work") {
		t.Fatalf("output listed another author's commit: %q", out)
	}
	if strings.Contains(out, "not implemented") || strings.Contains(out, "recorded") {
		t.Fatalf("dry-run output = %q", out)
	}

	db := openCLIStore(t, home)
	events, err := db.EventsByRepo(detectedRepo(t, out))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("dry-run stored %d events", len(events))
	}
}

func TestSyncRecordsDetectedCommits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, commitOnToday(-2*time.Minute), "me@example.com", "Me", "record on sync")
	writeSyncConfig(t, home, root)

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: recorded 1 commits") {
		t.Fatalf("output = %q, want recorded count", out)
	}

	db := openCLIStore(t, home)
	events, err := db.EventsByRepo(resolved(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Subject != "record on sync" {
		t.Fatalf("events = %+v", events)
	}

	again, err := execute(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("second dry-run: %v\n%s", err, again)
	}
	if strings.Contains(again, "record on sync") {
		t.Fatalf("dry-run after sync still listed the commit: %q", again)
	}
	if !strings.Contains(again, "sync: no commits") {
		t.Fatalf("second dry-run = %q, want no commits", again)
	}
}

// commitOnToday keeps a negative offset on today's local calendar day.
// sync uses time.Now(), so a commit a few minutes ago is yesterday when
// the test runs just after midnight. Git --since includes local midnight.
func commitOnToday(offset time.Duration) time.Time {
	now := time.Now()
	when := now.Add(offset)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if when.Before(start) {
		return start
	}
	return when
}

func writeSyncConfig(t *testing.T, home, root string) {
	t.Helper()
	body := "vault_path = " + quote(home) + "\nrole = \"writer\"\n\n[[roots]]\npath = " + quote(root) + "\nbucket = \"Hackastak\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n"
	writeConfigBody(t, home, body)
}

func quote(path string) string {
	return `"` + path + `"`
}

func detectedRepo(t *testing.T, out string) string {
	t.Helper()
	line := strings.TrimSpace(strings.Split(out, "\n")[0])
	fields := strings.Fields(line)
	if len(fields) < 2 {
		t.Fatalf("dry-run line = %q, want repo and hash", line)
	}
	return fields[0]
}

func openCLIStore(t *testing.T, home string) *store.DB {
	t.Helper()
	db, err := store.Open(defaultDB(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	got, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func gitCommitAt(t *testing.T, dir string, when time.Time, email, name, subject string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required")
	}
	cmd := exec.Command("git", "-c", "user.email="+email, "-c", "user.name="+name, "commit", "--allow-empty", "-m", subject)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gitconfig"),
		"GIT_AUTHOR_DATE="+when.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+when.Format(time.RFC3339),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit %q: %v\n%s", subject, err, out)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gitconfig"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
