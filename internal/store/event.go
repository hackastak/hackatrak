package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Kind is the kind of recorded activity.
type Kind string

const (
	KindCommit    Kind = "commit"
	KindFileTouch Kind = "file-touch"
)

// Event is one timestamped activity row. Hash is required for commits and
// empty for file-touch events. OccurredAt is stored in UTC.
type Event struct {
	ID          int64
	Kind        Kind
	Hash        string
	RepoPath    string
	ProjectSlug string
	OccurredAt  time.Time
	Branch      string
	Subject     string
	Body        string
	FileCount   int
	FileSample  string
}

// InsertEvent stores a new event. A repeated commit hash returns ErrDuplicate.
func (db *DB) InsertEvent(e Event) (Event, error) {
	if err := validateEvent(e); err != nil {
		return Event{}, fmt.Errorf("insert event: %w", err)
	}
	stored := e
	stored.OccurredAt = e.OccurredAt.UTC()

	var hash any
	if stored.Hash != "" {
		hash = stored.Hash
	}

	res, err := db.sql.Exec(`
		INSERT INTO events (
			kind, hash, repo_path, project_slug, occurred_at,
			branch, subject, body, file_count, file_sample
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(stored.Kind),
		hash,
		stored.RepoPath,
		stored.ProjectSlug,
		formatTime(stored.OccurredAt),
		stored.Branch,
		stored.Subject,
		stored.Body,
		stored.FileCount,
		stored.FileSample,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Event{}, fmt.Errorf("insert event %s: %w", stored.Hash, ErrDuplicate)
		}
		return Event{}, fmt.Errorf("insert event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("insert event: %w", err)
	}
	stored.ID = id
	return stored, nil
}

// EventByHash returns the commit event with the given hash.
func (db *DB) EventByHash(hash string) (Event, error) {
	if hash == "" {
		return Event{}, fmt.Errorf("event by hash: hash is required")
	}
	row := db.sql.QueryRow(eventSelect+` WHERE hash = ?`, hash)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, fmt.Errorf("event by hash %s: %w", hash, ErrNotFound)
	}
	if err != nil {
		return Event{}, fmt.Errorf("event by hash %s: %w", hash, err)
	}
	return event, nil
}

// EventsByRepo returns events for one repo, oldest first.
func (db *DB) EventsByRepo(repoPath string) ([]Event, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("events by repo: repo path is required")
	}
	rows, err := db.sql.Query(eventSelect+` WHERE repo_path = ? ORDER BY occurred_at, id`, repoPath)
	if err != nil {
		return nil, fmt.Errorf("events by repo %s: %w", repoPath, err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("events by repo %s: %w", repoPath, err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events by repo %s: %w", repoPath, err)
	}
	return events, nil
}

const eventSelect = `
	SELECT id, kind, hash, repo_path, project_slug, occurred_at,
		branch, subject, body, file_count, file_sample
	FROM events`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (Event, error) {
	var (
		event Event
		kind  string
		hash  sql.NullString
		at    string
	)
	err := row.Scan(
		&event.ID,
		&kind,
		&hash,
		&event.RepoPath,
		&event.ProjectSlug,
		&at,
		&event.Branch,
		&event.Subject,
		&event.Body,
		&event.FileCount,
		&event.FileSample,
	)
	if err != nil {
		return Event{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return Event{}, fmt.Errorf("parse occurred_at %q: %w", at, err)
	}
	event.Kind = Kind(kind)
	event.Hash = hash.String
	event.OccurredAt = parsed.UTC()
	return event, nil
}

func validateEvent(e Event) error {
	switch e.Kind {
	case KindCommit:
		if e.Hash == "" {
			return fmt.Errorf("commit hash is required")
		}
	case KindFileTouch:
		if e.Hash != "" {
			return fmt.Errorf("file-touch event must not have a commit hash")
		}
	default:
		return fmt.Errorf("kind %q is not supported", e.Kind)
	}
	if e.RepoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code()
	return code == sqlite3.SQLITE_CONSTRAINT || code&0xff == sqlite3.SQLITE_CONSTRAINT
}
