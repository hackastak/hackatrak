package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SyncState is the last-run cursor for one repository.
// Cursor is opaque to the store. The scanner decides the format.
type SyncState struct {
	RepoPath  string
	Cursor    string
	UpdatedAt time.Time
}

// SyncState returns the cursor for repoPath. A missing row is ErrNotFound.
func (db *DB) SyncState(repoPath string) (SyncState, error) {
	if repoPath == "" {
		return SyncState{}, fmt.Errorf("sync state: repo path is required")
	}
	var state SyncState
	var updated string
	err := db.sql.QueryRow(
		`SELECT repo_path, cursor, updated_at FROM sync_state WHERE repo_path = ?`,
		repoPath,
	).Scan(&state.RepoPath, &state.Cursor, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncState{}, fmt.Errorf("sync state %s: %w", repoPath, ErrNotFound)
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("sync state %s: %w", repoPath, err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return SyncState{}, fmt.Errorf("sync state %s: %w", repoPath, err)
	}
	state.UpdatedAt = parsed.UTC()
	return state, nil
}

// SetSyncCursor inserts or replaces the cursor for repoPath.
func (db *DB) SetSyncCursor(repoPath, cursor string) (SyncState, error) {
	if repoPath == "" {
		return SyncState{}, fmt.Errorf("set sync cursor: repo path is required")
	}
	if cursor == "" {
		return SyncState{}, fmt.Errorf("set sync cursor: cursor is required")
	}
	updated := time.Now().UTC()
	_, err := db.sql.Exec(`
		INSERT INTO sync_state (repo_path, cursor, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_path) DO UPDATE SET
			cursor = excluded.cursor,
			updated_at = excluded.updated_at`,
		repoPath, cursor, formatTime(updated),
	)
	if err != nil {
		return SyncState{}, fmt.Errorf("set sync cursor %s: %w", repoPath, err)
	}
	return SyncState{RepoPath: repoPath, Cursor: cursor, UpdatedAt: updated}, nil
}
