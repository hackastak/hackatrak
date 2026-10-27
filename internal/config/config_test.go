package config

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfigDecodesPlannedFields(t *testing.T) {
	const raw = `
vault_path = "/Users/hackastak/Notes"
weekly_glob = "_Weekly/{YYYY}/{YYYY}-W{ww}.md"
account = "hackastak"
role = "writer"
spool_dir = "_Weekly/.hackatrak"
ticket_regex = "NEXTGEN-\\d+"
exclude_globs = ["*.swp", "*~"]
summarizer = "mechanical"

[[roots]]
path = "/Users/hackastak/Developer"
bucket = "Hackastak"

[[roots]]
path = "/Users/i568439/Developer"
bucket = "SAP Business Network & ANT"

[git]
author_emails = ["me@example.com", "work@example.com"]
`
	var cfg Config
	if err := toml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.VaultPath != "/Users/hackastak/Notes" {
		t.Errorf("VaultPath = %q", cfg.VaultPath)
	}
	if cfg.WeeklyGlob != DefaultWeeklyGlob {
		t.Errorf("WeeklyGlob = %q, want %q", cfg.WeeklyGlob, DefaultWeeklyGlob)
	}
	if cfg.Account != "hackastak" {
		t.Errorf("Account = %q", cfg.Account)
	}
	if cfg.Role != RoleWriter {
		t.Errorf("Role = %q, want %q", cfg.Role, RoleWriter)
	}
	if cfg.SpoolDir != "_Weekly/.hackatrak" {
		t.Errorf("SpoolDir = %q", cfg.SpoolDir)
	}
	if cfg.TicketRegex != `NEXTGEN-\d+` {
		t.Errorf("TicketRegex = %q", cfg.TicketRegex)
	}
	if cfg.Summarizer != SummarizerMechanical {
		t.Errorf("Summarizer = %q, want %q", cfg.Summarizer, SummarizerMechanical)
	}
	if len(cfg.ExcludeGlobs) != 2 || cfg.ExcludeGlobs[0] != "*.swp" || cfg.ExcludeGlobs[1] != "*~" {
		t.Errorf("ExcludeGlobs = %#v", cfg.ExcludeGlobs)
	}
	if len(cfg.Roots) != 2 {
		t.Fatalf("Roots = %#v, want 2", cfg.Roots)
	}
	if cfg.Roots[0].Path != "/Users/hackastak/Developer" || cfg.Roots[0].Bucket != "Hackastak" {
		t.Errorf("Roots[0] = %#v", cfg.Roots[0])
	}
	if cfg.Roots[1].Bucket != "SAP Business Network & ANT" {
		t.Errorf("Roots[1].Bucket = %q", cfg.Roots[1].Bucket)
	}
	if len(cfg.Git.AuthorEmails) != 2 || cfg.Git.AuthorEmails[1] != "work@example.com" {
		t.Errorf("Git.AuthorEmails = %#v", cfg.Git.AuthorEmails)
	}
}
