package config

import (
	"os"
	"strings"
	"testing"
)

func TestReadmeConfigReference(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	text := string(raw)
	start := strings.Index(text, "## Configuration\n")
	end := strings.Index(text, "## Development\n")
	if start < 0 || end < start {
		t.Fatal("README is missing a Configuration section before Development")
	}
	section := text[start:end]
	for _, field := range []string{
		"vault_path",
		"weekly_glob",
		"account",
		"role",
		"spool_dir",
		"ticket_regex",
		"exclude_globs",
		"summarizer",
		"roots",
		"path",
		"bucket",
		"author_emails",
		"writer",
		"spooler",
		"Required",
		"Default",
		"~/.config/hackatrak/config.toml",
		"~/.local/state/hackatrak/hackatrak.db",
	} {
		if !strings.Contains(section, field) {
			t.Errorf("Configuration section does not document %q", field)
		}
	}
}
