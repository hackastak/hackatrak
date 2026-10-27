package main

import (
	"bytes"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	root.SilenceUsage = true
	root.SilenceErrors = true
	err := root.Execute()
	return buf.String(), err
}

func TestRootRegistersPhase0Subcommands(t *testing.T) {
	root := newRootCmd()
	got := map[string]bool{}
	for _, cmd := range root.Commands() {
		got[cmd.Name()] = true
	}
	for _, name := range []string{"sync", "today", "summary", "log", "install", "version"} {
		if !got[name] {
			t.Errorf("root is missing subcommand %q", name)
		}
	}
}

func TestStubSubcommandsPrintPlaceholder(t *testing.T) {
	for _, name := range []string{"sync", "today", "summary", "log", "install"} {
		t.Run(name, func(t *testing.T) {
			out, err := execute(t, name)
			if err != nil {
				t.Fatalf("execute() error = %v", err)
			}
			want := "hackatrak " + name + ": not implemented\n"
			if out != want {
				t.Errorf("output = %q, want %q", out, want)
			}
		})
	}
}

func TestVersionPrintsEmbeddedBuildInfo(t *testing.T) {
	origVersion, origCommit := version, commit
	t.Cleanup(func() {
		version = origVersion
		commit = origCommit
	})
	version = "v0.0.0-test"
	commit = "abc123"

	out, err := execute(t, "version")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "v0.0.0-test") || !strings.Contains(out, "abc123") {
		t.Errorf("output = %q, want version and commit", out)
	}
	if strings.Contains(out, "not implemented") {
		t.Errorf("version is still a stub: %q", out)
	}
}

func TestVersionDefaultsMatchLdflagsTargets(t *testing.T) {
	if version != "dev" {
		t.Errorf("version = %q, want %q (ldflags -X main.version)", version, "dev")
	}
	if commit != "none" {
		t.Errorf("commit = %q, want %q (ldflags -X main.commit)", commit, "none")
	}
}

func TestUnknownCommandFails(t *testing.T) {
	_, err := execute(t, "no-such")
	if err == nil {
		t.Fatal("expected unknown command to fail")
	}
}
