package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestExampleDocumentsEveryField(t *testing.T) {
	path := filepath.Join("..", "..", "config.example.toml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	var cfg Config
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		t.Fatalf("decode example: %v", err)
	}
	if unknown := unknownFields(md); len(unknown) > 0 {
		t.Fatalf("example has unknown fields: %v", unknown)
	}

	defined := [][]string{
		{"vault_path"},
		{"weekly_glob"},
		{"account"},
		{"role"},
		{"spool_dir"},
		{"ticket_regex"},
		{"exclude_globs"},
		{"summarizer"},
		{"roots"},
		{"git", "author_emails"},
	}
	for _, key := range defined {
		if !md.IsDefined(key...) {
			t.Errorf("example does not set %s", strings.Join(key, "."))
		}
	}
	if len(cfg.Roots) == 0 || cfg.Roots[0].Path == "" || cfg.Roots[0].Bucket == "" {
		t.Errorf("example roots = %#v, want path and bucket", cfg.Roots)
	}
	if cfg.Role != RoleWriter && cfg.Role != RoleSpooler {
		t.Errorf("example role = %q", cfg.Role)
	}
	if cfg.Summarizer != SummarizerMechanical && cfg.Summarizer != SummarizerOllama {
		t.Errorf("example summarizer = %q", cfg.Summarizer)
	}

	text := string(raw)
	for _, field := range []string{
		"vault_path",
		"weekly_glob",
		"account",
		"role",
		"spool_dir",
		"ticket_regex",
		"exclude_globs",
		"summarizer",
		"path",
		"bucket",
		"author_emails",
	} {
		if !commentMentions(text, field) {
			t.Errorf("example has no comment documenting %q", field)
		}
	}
}

func commentMentions(text, field string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && hasWord(trimmed, field) {
			return true
		}
	}
	return false
}

func hasWord(line, field string) bool {
	for _, word := range strings.FieldsFunc(line, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_'
	}) {
		if word == field {
			return true
		}
	}
	return false
}
