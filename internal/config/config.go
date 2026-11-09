// Package config loads and validates ~/.config/hackatrak/config.toml.
package config

// DefaultWeeklyGlob is the weekly-note path relative to the vault.
// Placeholders are filled when the week file is resolved, not during load.
const DefaultWeeklyGlob = "_Weekly/{YYYY}/{YYYY}-W{ww}.md"

// DefaultSpoolDir is the vault-relative directory for cross-account spool files.
// Load joins it to vault_path. The directory does not need to exist yet.
const DefaultSpoolDir = "_Weekly/.hackatrak"

// Role is which account this binary is running as.
type Role string

const (
	RoleWriter  Role = "writer"
	RoleSpooler Role = "spooler"
)

// Summarizer selects how a day's "what was done" line is built.
type Summarizer string

const (
	SummarizerMechanical Summarizer = "mechanical"
	SummarizerOllama     Summarizer = "ollama"
)

// Root maps a filesystem prefix to a weekly-note life bucket.
type Root struct {
	Path   string `toml:"path"`
	Bucket string `toml:"bucket"`
}

// Git holds the author filter used when scanning commits.
type Git struct {
	AuthorEmails []string `toml:"author_emails"`
}

// Config is the on-disk configuration. The zero value is empty.
// Load applies defaults and ~ expansion, resolves spool_dir under the vault,
// then Validate checks the result.
//
// Account and Role are the cross-account handoff identity. Bucket is
// roots[].bucket: one account may map several roots to different headings.
// A writer splices the weekly note. A spooler writes only its spool.
type Config struct {
	VaultPath    string     `toml:"vault_path"`
	WeeklyGlob   string     `toml:"weekly_glob"`
	Roots        []Root     `toml:"roots"`
	Account      string     `toml:"account"`
	Role         Role       `toml:"role"`
	SpoolDir     string     `toml:"spool_dir"`
	Git          Git        `toml:"git"`
	TicketRegex  string     `toml:"ticket_regex"`
	ExcludeGlobs []string   `toml:"exclude_globs"`
	Summarizer   Summarizer `toml:"summarizer"`
}
