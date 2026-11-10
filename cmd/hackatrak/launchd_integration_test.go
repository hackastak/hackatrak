//go:build darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/launchd"
	"github.com/hackastak/hackatrak/internal/vault"
)

// TestPhase8 loads a throwaway LaunchAgent and launchctl-starts it.
// The job's HOME is a temp directory, so the scheduled sync cannot write
// the real vault. Catch-up after sleep is the launchd contract for a
// StartCalendarInterval plist with no TimeZone key: a missed 12:00 or 23:00
// runs once on wake. This test cannot sleep the machine; it checks that the
// loaded job is that plist, then runs the same program with launchctl start.
func TestPhase8(t *testing.T) {
	if os.Getenv("HACKATRAK_SKIP_LAUNCHD") == "1" {
		t.Skip("HACKATRAK_SKIP_LAUNCHD=1")
	}
	domain := launchd.Domain()
	if out, err := launchd.Exec(context.Background(), "launchctl", "print", domain); err != nil {
		t.Skipf("gui domain unavailable (%s): %v\n%s", domain, err, out)
	}

	home := t.TempDir()
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, time.Now().Add(-time.Minute), "me@example.com", "Me", "launchd scheduled sync")

	bin := buildHackatrak(t)
	agent := launchd.DefaultAgent(home, bin)
	agent.Label = launchd.Label + ".ptest"
	agent.PlistPath = filepath.Join(home, "Library", "LaunchAgents", agent.Label+".plist")
	agent.Env = map[string]string{
		"HOME": home,
		"PATH": os.Getenv("PATH"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		if err := launchd.Uninstall(cleanCtx, agent, nil); err != nil {
			t.Errorf("cleanup uninstall: %v", err)
		}
	})

	if _, err := launchd.Install(ctx, agent, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("note existed before launchctl start: %v", err)
	}

	if err := launchd.Start(ctx, agent, nil); err != nil {
		t.Fatalf("launchctl start: %v", err)
	}

	var note, stdoutLog, stderrLog string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if body, err := os.ReadFile(notePath); err == nil {
			note = string(body)
		}
		if body, err := os.ReadFile(agent.Stdout); err == nil {
			stdoutLog = string(body)
		}
		if body, err := os.ReadFile(agent.Stderr); err == nil {
			stderrLog = string(body)
		}
		if strings.Contains(note, "<!-- ht:demo -->") && strings.Contains(stdoutLog, "sync:") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !strings.Contains(note, "<!-- ht:demo -->") || !strings.Contains(note, "launchd scheduled sync") {
		t.Fatalf("scheduled run did not write the note\nstdout:\n%s\nstderr:\n%s\nnote:\n%s", stdoutLog, stderrLog, note)
	}
	if !strings.Contains(stdoutLog, "sync: wrote") {
		t.Fatalf("stdout log missing sync output:\n%s\nstderr:\n%s", stdoutLog, stderrLog)
	}

	printed, err := launchd.Print(ctx, agent, nil)
	if err != nil {
		t.Fatalf("launchctl print: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, agent.PlistPath) {
		t.Fatalf("loaded job is not the test plist:\n%s", printed)
	}
	loaded, err := os.ReadFile(agent.PlistPath)
	if err != nil {
		t.Fatal(err)
	}
	loadedText := string(loaded)
	if strings.Contains(loadedText, "<key>TimeZone</key>") || !strings.Contains(loadedText, "StartCalendarInterval") {
		t.Fatalf("loaded plist is not a local calendar schedule:\n%s", loadedText)
	}
	for _, hour := range []string{"<integer>12</integer>", "<integer>23</integer>"} {
		if !strings.Contains(loadedText, hour) {
			t.Fatalf("loaded plist missing %s\n%s", hour, loadedText)
		}
	}
	if runs, ok := launchdRuns(printed); ok && runs < 1 {
		t.Fatalf("launchctl print runs = %d, want at least 1\n%s", runs, printed)
	}
}

func launchdRuns(printed string) (int, bool) {
	for _, line := range strings.Split(printed, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "runs = ") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(line, "runs = "))
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
