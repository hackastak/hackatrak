package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PresenceSource is why a project-day counts as worked.
type PresenceSource string

const (
	// PresenceCommit means the day has at least one commit.
	PresenceCommit PresenceSource = "commit"
	// PresenceFile means the day has file touches and no commits.
	PresenceFile PresenceSource = "file"
)

// hashListSep joins commit hashes in the commit_hashes column.
// Hashes are single tokens, so a newline cannot appear in a stored hash.
const hashListSep = "\n"

// DayPresence is one derived project-day row.
// CommitHashes keeps caller order. Ticket may be empty; summarize fills it later.
type DayPresence struct {
	ID           int64
	ProjectSlug  string
	Day          string // YYYY-MM-DD
	Worked       bool
	Source       PresenceSource
	Branch       string
	Ticket       string
	CommitHashes []string
	FileCount    int
	FileSample   string
}

// UpsertDayPresence inserts or replaces the row for (project, day).
//
// A second call with the same project and day keeps the row id and replaces
// every presence column, including clearing branch, ticket, and commit hashes
// when the new row omits them. Other project-days are left alone.
// The input hash slice is not modified.
func (db *DB) UpsertDayPresence(row DayPresence) (DayPresence, error) {
	stored, err := normalizeDayPresence(row)
	if err != nil {
		return DayPresence{}, fmt.Errorf("upsert day presence: %w", err)
	}
	hashes, err := formatHashes(stored.CommitHashes)
	if err != nil {
		return DayPresence{}, fmt.Errorf("upsert day presence: %w", err)
	}

	worked := 0
	if stored.Worked {
		worked = 1
	}
	_, err = db.sql.Exec(`
		INSERT INTO day_presence (
			project_slug, day, worked, source, branch, ticket,
			commit_hashes, file_count, file_sample
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_slug, day) DO UPDATE SET
			worked = excluded.worked,
			source = excluded.source,
			branch = excluded.branch,
			ticket = excluded.ticket,
			commit_hashes = excluded.commit_hashes,
			file_count = excluded.file_count,
			file_sample = excluded.file_sample`,
		stored.ProjectSlug,
		stored.Day,
		worked,
		string(stored.Source),
		stored.Branch,
		stored.Ticket,
		hashes,
		stored.FileCount,
		stored.FileSample,
	)
	if err != nil {
		return DayPresence{}, fmt.Errorf("upsert day presence %s %s: %w", stored.ProjectSlug, stored.Day, err)
	}
	saved, err := db.DayPresence(stored.ProjectSlug, stored.Day)
	if err != nil {
		return DayPresence{}, fmt.Errorf("upsert day presence %s %s: %w", stored.ProjectSlug, stored.Day, err)
	}
	return saved, nil
}

// DayPresence returns the row for one project on one calendar day.
// A missing row is ErrNotFound.
func (db *DB) DayPresence(projectSlug, day string) (DayPresence, error) {
	slug := strings.TrimSpace(projectSlug)
	if slug == "" {
		return DayPresence{}, fmt.Errorf("day presence: project slug is required")
	}
	if err := validateDay(day); err != nil {
		return DayPresence{}, fmt.Errorf("day presence: %w", err)
	}

	row, err := scanDayPresence(db.sql.QueryRow(presenceSelect+` WHERE project_slug = ? AND day = ?`, slug, day))
	if errors.Is(err, sql.ErrNoRows) {
		return DayPresence{}, fmt.Errorf("day presence %s %s: %w", slug, day, ErrNotFound)
	}
	if err != nil {
		return DayPresence{}, fmt.Errorf("day presence %s %s: %w", slug, day, err)
	}
	return row, nil
}

// DayPresences returns rows whose day is from through to, inclusive.
// Days are YYYY-MM-DD. Results are ordered by day, then project slug.
// An empty range returns nil, not an empty slice. from after to is an error.
func (db *DB) DayPresences(from, to string) ([]DayPresence, error) {
	if err := validateDay(from); err != nil {
		return nil, fmt.Errorf("day presences: %w", err)
	}
	if err := validateDay(to); err != nil {
		return nil, fmt.Errorf("day presences: %w", err)
	}
	if from > to {
		return nil, fmt.Errorf("day presences: from %s is after %s", from, to)
	}

	rows, err := db.sql.Query(presenceSelect+` WHERE day >= ? AND day <= ? ORDER BY day, project_slug`, from, to)
	if err != nil {
		return nil, fmt.Errorf("day presences %s..%s: %w", from, to, err)
	}
	defer rows.Close()

	var found []DayPresence
	for rows.Next() {
		row, err := scanDayPresence(rows)
		if err != nil {
			return nil, fmt.Errorf("day presences %s..%s: %w", from, to, err)
		}
		found = append(found, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("day presences %s..%s: %w", from, to, err)
	}
	return found, nil
}

const presenceSelect = `
	SELECT id, project_slug, day, worked, source, branch, ticket,
		commit_hashes, file_count, file_sample
	FROM day_presence`

func scanDayPresence(row rowScanner) (DayPresence, error) {
	var (
		presence DayPresence
		worked   int
		source   string
		hashes   string
	)
	err := row.Scan(
		&presence.ID,
		&presence.ProjectSlug,
		&presence.Day,
		&worked,
		&source,
		&presence.Branch,
		&presence.Ticket,
		&hashes,
		&presence.FileCount,
		&presence.FileSample,
	)
	if err != nil {
		return DayPresence{}, err
	}
	presence.Worked = worked != 0
	presence.Source = PresenceSource(source)
	presence.CommitHashes = parseHashes(hashes)
	return presence, nil
}

func normalizeDayPresence(row DayPresence) (DayPresence, error) {
	row.ProjectSlug = strings.TrimSpace(row.ProjectSlug)
	row.Day = strings.TrimSpace(row.Day)
	if row.ProjectSlug == "" {
		return DayPresence{}, fmt.Errorf("project slug is required")
	}
	if err := validateDay(row.Day); err != nil {
		return DayPresence{}, err
	}
	switch row.Source {
	case PresenceCommit, PresenceFile:
	default:
		return DayPresence{}, fmt.Errorf("source %q is not supported", row.Source)
	}
	if row.FileCount < 0 {
		return DayPresence{}, fmt.Errorf("file count must not be negative")
	}
	hashes, err := checkedHashes(row.CommitHashes)
	if err != nil {
		return DayPresence{}, err
	}
	if row.Source == PresenceCommit && len(hashes) == 0 {
		return DayPresence{}, fmt.Errorf("commit presence requires a commit hash")
	}
	if row.Source == PresenceFile && len(hashes) > 0 {
		return DayPresence{}, fmt.Errorf("file presence must not include commit hashes")
	}
	row.CommitHashes = hashes
	return row, nil
}

func validateDay(day string) error {
	if day == "" {
		return fmt.Errorf("day is required")
	}
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil || parsed.Format("2006-01-02") != day {
		return fmt.Errorf("day %q must be YYYY-MM-DD", day)
	}
	return nil
}

func checkedHashes(hashes []string) ([]string, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(hashes))
	out := make([]string, 0, len(hashes))
	for i, hash := range hashes {
		if hash == "" || strings.TrimSpace(hash) != hash || strings.Contains(hash, hashListSep) {
			return nil, fmt.Errorf("commit hash %d is invalid", i)
		}
		if _, ok := seen[hash]; ok {
			return nil, fmt.Errorf("commit hash %q is duplicated", hash)
		}
		seen[hash] = struct{}{}
		out = append(out, hash)
	}
	return out, nil
}

func formatHashes(hashes []string) (string, error) {
	checked, err := checkedHashes(hashes)
	if err != nil {
		return "", err
	}
	return strings.Join(checked, hashListSep), nil
}

func parseHashes(stored string) []string {
	if stored == "" {
		return nil
	}
	return strings.Split(stored, hashListSep)
}
