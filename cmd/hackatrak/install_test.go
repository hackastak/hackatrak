package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hackastak/hackatrak/internal/launchd"
)

func TestInstallCommandWritesAgentWithoutOpeningStore(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("install is macOS-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "bin", "hackatrak")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	restoreRunner(t, func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return "", nil
	})

	out, err := execute(t, "install", "--binary", bin)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "schedule 12:00 and 23:00 local") {
		t.Fatalf("output = %q", out)
	}
	plist := launchd.PlistPath(home)
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<string>sync</string>") || !strings.Contains(string(body), bin) {
		t.Fatalf("plist = %s", body)
	}
	if _, err := os.Stat(defaultDB(home)); !os.IsNotExist(err) {
		t.Fatalf("install opened the store: %v", err)
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "launchctl bootstrap "+launchd.Domain()+" "+plist) {
		t.Fatalf("calls = %s", joined)
	}
}

func TestUninstallCommandDoesNotNeedConfig(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("uninstall is macOS-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	agent := launchd.DefaultAgent(home, filepath.Join(home, "hackatrak"))
	if err := os.MkdirAll(filepath.Dir(agent.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.PlistPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	restoreRunner(t, func(_ context.Context, name string, args ...string) (string, error) {
		return "", nil
	})

	out, err := execute(t, "uninstall")
	if err != nil {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}
	if !strings.Contains(out, "unloaded "+agent.Service()) {
		t.Fatalf("output = %q", out)
	}
	if _, err := os.Stat(agent.PlistPath); !os.IsNotExist(err) {
		t.Fatalf("plist still present: %v", err)
	}
}

func TestInstallRejectsOtherPlatforms(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin has a real install path")
	}
	_, err := execute(t, "install")
	if err == nil || !strings.Contains(err.Error(), "macOS-only") {
		t.Fatalf("error = %v", err)
	}
}

func restoreRunner(t *testing.T, runner launchd.Runner) {
	t.Helper()
	orig := launchRunner
	launchRunner = runner
	t.Cleanup(func() { launchRunner = orig })
}
