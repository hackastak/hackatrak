package launchd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type scripted struct {
	calls []string
	fail  map[string]error
}

func (s *scripted) run(_ context.Context, name string, args ...string) (string, error) {
	line := name + " " + strings.Join(args, " ")
	s.calls = append(s.calls, line)
	if len(args) > 0 {
		if err := s.fail[args[0]]; err != nil {
			return "service not loaded", err
		}
	}
	return "", nil
}

func testAgent(t *testing.T) Agent {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, "hackatrak")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	agent := DefaultAgent(home, bin)
	agent.Label = Label + ".test"
	agent.PlistPath = filepath.Join(home, "Library", "LaunchAgents", agent.Label+".plist")
	return agent
}

func TestInstallBootstrapsAndWritesPlist(t *testing.T) {
	agent := testAgent(t)
	script := &scripted{}
	method, err := Install(context.Background(), agent, script.run)
	if err != nil {
		t.Fatal(err)
	}
	if method != MethodBootstrap {
		t.Fatalf("method = %s, want bootstrap", method)
	}
	body, err := os.ReadFile(agent.PlistPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "<string>"+agent.Program+"</string>") || !strings.Contains(text, "<string>sync</string>") {
		t.Fatalf("plist program arguments:\n%s", text)
	}
	if !strings.Contains(text, "<integer>12</integer>") || !strings.Contains(text, "<integer>23</integer>") {
		t.Fatalf("plist schedule:\n%s", text)
	}
	if _, err := os.Stat(filepath.Dir(agent.Stdout)); err != nil {
		t.Fatalf("log dir: %v", err)
	}
	joined := strings.Join(script.calls, "\n")
	for _, want := range []string{
		"launchctl enable " + agent.Service(),
		"launchctl bootout " + agent.Service(),
		"launchctl bootstrap " + Domain() + " " + agent.PlistPath,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("calls missing %q\n%s", want, joined)
		}
	}
}

func TestInstallFallsBackToLoad(t *testing.T) {
	agent := testAgent(t)
	script := &scripted{fail: map[string]error{"bootstrap": errors.New("unrecognized subcommand")}}
	method, err := Install(context.Background(), agent, script.run)
	if err != nil {
		t.Fatal(err)
	}
	if method != MethodLoad {
		t.Fatalf("method = %s, want load", method)
	}
	joined := strings.Join(script.calls, "\n")
	if !strings.Contains(joined, "launchctl load -w "+agent.PlistPath) {
		t.Fatalf("calls = %s", joined)
	}
}

func TestInstallReportsBothLaunchctlFailures(t *testing.T) {
	agent := testAgent(t)
	script := &scripted{fail: map[string]error{
		"bootstrap": errors.New("domain unavailable"),
		"load":      errors.New("load failed"),
	}}
	_, err := Install(context.Background(), agent, script.run)
	if err == nil {
		t.Fatal("expected launchctl failure")
	}
	if _, statErr := os.Stat(agent.PlistPath); statErr != nil {
		t.Fatalf("plist removed after launchctl failure: %v", statErr)
	}
}

func TestInstallRejectsEphemeralAndNonExecutable(t *testing.T) {
	agent := testAgent(t)
	agent.Program = filepath.Join(string(filepath.Separator), "var", "folders", "go-build", "b001", "hackatrak")
	if _, err := Install(context.Background(), agent, (&scripted{}).run); err == nil {
		t.Fatal("ephemeral binary was accepted")
	}

	if runtime.GOOS == "windows" {
		return
	}
	agent = testAgent(t)
	if err := os.Chmod(agent.Program, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), agent, (&scripted{}).run); err == nil {
		t.Fatal("non-executable binary was accepted")
	}
}

func TestUninstallBootsOutAndRemovesPlist(t *testing.T) {
	agent := testAgent(t)
	if err := os.MkdirAll(filepath.Dir(agent.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.PlistPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &scripted{}
	if err := Uninstall(context.Background(), agent, script.run); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(agent.PlistPath); !os.IsNotExist(err) {
		t.Fatalf("plist still present: %v", err)
	}
	if len(script.calls) != 1 || !strings.Contains(script.calls[0], "bootout "+agent.Service()) {
		t.Fatalf("calls = %#v", script.calls)
	}
}

func TestUninstallIsIdempotentWhenMissing(t *testing.T) {
	agent := testAgent(t)
	script := &scripted{fail: map[string]error{
		"bootout": errors.New("Boot-out failed: 3: No such process"),
		"unload":  errors.New("Unload failed: 5: Input/output error"),
	}}
	if err := Uninstall(context.Background(), agent, script.run); err != nil {
		t.Fatal(err)
	}
}

func TestServiceMissingDoesNotTreatEveryErrorAsGone(t *testing.T) {
	if serviceMissing("Bootstrap failed: 5: Input/output error") {
		t.Fatal("I/O error was treated as a missing service")
	}
	if !serviceMissing("Boot-out failed: 3: No such process") {
		t.Fatal("missing service was not recognized")
	}
}
