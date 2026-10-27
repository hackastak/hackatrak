package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "hackatrak", "config.toml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := expandHome("~/Developer")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "Developer") {
		t.Errorf("expandHome(~/Developer) = %q", got)
	}

	got, err = expandHome("~")
	if err != nil {
		t.Fatal(err)
	}
	if got != home {
		t.Errorf("expandHome(~) = %q, want %q", got, home)
	}

	got, err = expandHome("/already/absolute")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/already/absolute" {
		t.Errorf("expandHome(absolute) = %q", got)
	}

	if _, err = expandHome("~other/path"); err == nil {
		t.Fatal("expandHome(~other) error = nil, want an error")
	}
}

func TestLoadExpandsTildeAndAppliesDefaults(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := writeConfig(t, dir, `
vault_path = "~"
account = "hackastak"
role = "writer"
spool_dir = "~/.config"
ticket_regex = "NEXTGEN-\\d+"

[[roots]]
path = "~"
bucket = "Hackastak"

[git]
author_emails = ["me@example.com"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.VaultPath != home {
		t.Errorf("VaultPath = %q, want %q", cfg.VaultPath, home)
	}
	if cfg.Roots[0].Path != home {
		t.Errorf("Roots[0].Path = %q, want %q", cfg.Roots[0].Path, home)
	}
	if cfg.SpoolDir != filepath.Join(home, ".config") {
		t.Errorf("SpoolDir = %q", cfg.SpoolDir)
	}
	if cfg.WeeklyGlob != DefaultWeeklyGlob {
		t.Errorf("WeeklyGlob = %q, want default", cfg.WeeklyGlob)
	}
	if cfg.Summarizer != SummarizerMechanical {
		t.Errorf("Summarizer = %q, want %q", cfg.Summarizer, SummarizerMechanical)
	}
}

func TestLoadMalformedIncludesLine(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "vault_path = \"/tmp\"\nrole = \"writer\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want a parse error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "line") {
		t.Errorf("error = %q, want a line number", msg)
	}
	if !strings.Contains(msg, path) {
		t.Errorf("error = %q, want the config path", msg)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `
vault_path = "~"
account = "hackastak"
role = "writer"
spool_dir = "~"
not_a_field = true

[[roots]]
path = "~"
bucket = "Hackastak"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want unknown field")
	}
	if !strings.Contains(err.Error(), "not_a_field") {
		t.Errorf("error = %q, want the unknown field", err.Error())
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("Load() error = nil, want missing file")
	}
}

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
