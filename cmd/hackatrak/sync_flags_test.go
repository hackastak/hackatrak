package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncDateLimitsTheDryRunWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "main")
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	gitCommitAt(t, repo, yesterday, "me@example.com", "Me", "yesterday only")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "today only")
	writeSyncConfig(t, home, root)

	today := now.Format("2006-01-02")
	out, err := execute(t, "sync", "--dry-run", "--date", today)
	if err != nil {
		t.Fatalf("sync --date today: %v\n%s", err, out)
	}
	if !strings.Contains(out, "today only") || strings.Contains(out, "yesterday only") {
		t.Fatalf("today window = %q", out)
	}

	out, err = execute(t, "sync", "--dry-run", "--date", yesterday.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("sync --date yesterday: %v\n%s", err, out)
	}
	if !strings.Contains(out, "yesterday only") || strings.Contains(out, "today only") {
		t.Fatalf("yesterday window = %q", out)
	}
	db := openCLIStore(t, home)
	events, err := db.EventsByRepo(resolved(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("dated dry-run stored events: %+v", events)
	}
}

func TestSyncDateRejectsUnsafeValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "sync", "--date", "11/10/2026")
	if err == nil || !errors.As(err, new(*usageError)) {
		t.Fatalf("bad date = %v\n%s", err, out)
	}
	future := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	out, err = execute(t, "sync", "--date", future)
	if err == nil || !errors.As(err, new(*usageError)) {
		t.Fatalf("future date = %v\n%s", err, out)
	}
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	out, err = execute(t, "sync", "--date", past)
	if err == nil || !errors.As(err, new(*usageError)) || !strings.Contains(err.Error(), "dry-run") {
		t.Fatalf("past write = %v\n%s", err, out)
	}
}

func TestSyncVerboseLogsTheDay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if strings.Contains(out, "sync: day ") {
		t.Fatalf("quiet sync logged the day:\n%s", out)
	}
	out, err = execute(t, "sync", "--dry-run", "--verbose")
	if err != nil {
		t.Fatalf("verbose: %v\n%s", err, out)
	}
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(out, "sync: day "+today) || !strings.Contains(out, "sync: mode dry-run") {
		t.Fatalf("verbose output = %q", out)
	}
}

func TestSyncExitCodes(t *testing.T) {
	bin := buildHackatrak(t)

	t.Run("success", func(t *testing.T) {
		home := t.TempDir()
		writeValidConfig(t, home)
		out, err := runHackatrak(t, bin, home, "sync", "--dry-run")
		if err != nil {
			t.Fatalf("exit = %v\n%s", err, out)
		}
	})
	t.Run("usage", func(t *testing.T) {
		home := t.TempDir()
		writeValidConfig(t, home)
		assertExit(t, bin, home, 2, "sync", "--date", "nope")
		assertExit(t, bin, home, 2, "sync", "--not-a-flag")
		assertExit(t, bin, home, 2, "nope")
	})
	t.Run("config", func(t *testing.T) {
		home := t.TempDir()
		body := "vault_path = " + quote(home) + "\nrole = \"nope\"\naccount = \"hackastak\"\n\n[[roots]]\npath = " + quote(home) + "\nbucket = \"Hackastak\"\n"
		writeConfigBody(t, home, body)
		assertExit(t, bin, home, 1, "sync")
	})
}

func assertExit(t *testing.T, bin, home string, want int, args ...string) {
	t.Helper()
	out, err := runHackatrak(t, bin, home, args...)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != want {
		t.Fatalf("hackatrak %s exit = %v, want %d\n%s", strings.Join(args, " "), err, want, out)
	}
}
