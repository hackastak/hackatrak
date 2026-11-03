package scan

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// RepoSince returns the git --since bound for repoPath.
//
// A stored sync_state cursor is returned unchanged. No row means the start
// of now's calendar day in now's location, RFC3339 — the same fallback
// CommitsSince uses for an empty since. Reading does not write a cursor.
// now must be non-zero so that fallback is deterministic.
//
// repoPath is the key DiscoverRepos returns. It is trimmed, not resolved.
func RepoSince(db *store.DB, repoPath string, now time.Time) (string, error) {
	repoPath, err := cursorRepoPath(repoPath)
	if err != nil {
		return "", fmt.Errorf("repo since: %w", err)
	}
	if err := cursorArgs(db, now); err != nil {
		return "", fmt.Errorf("repo since: %w", err)
	}

	state, err := db.SyncState(repoPath)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return LocalMidnight(now).Format(time.RFC3339), nil
		}
		return "", fmt.Errorf("repo since: %w", err)
	}
	cursor := strings.TrimSpace(state.Cursor)
	if cursor == "" {
		return "", fmt.Errorf("repo since: %s: cursor is empty", repoPath)
	}
	return cursor, nil
}

// AdvanceRepoCursor records now as the last-run cursor for repoPath.
//
// The stored value is now in its own location, RFC3339 at second resolution,
// so the next RepoSince can pass it straight to git log --since. Git's
// --since includes a commit in that same second; hash dedup covers the
// boundary. A later advance replaces this row. Other repos are left alone.
//
// Call this only after the scan that used RepoSince has persisted. Advancing
// first would skip commits written between the two calls.
func AdvanceRepoCursor(db *store.DB, repoPath string, now time.Time) (store.SyncState, error) {
	repoPath, err := cursorRepoPath(repoPath)
	if err != nil {
		return store.SyncState{}, fmt.Errorf("advance repo cursor: %w", err)
	}
	if err := cursorArgs(db, now); err != nil {
		return store.SyncState{}, fmt.Errorf("advance repo cursor: %w", err)
	}

	state, err := db.SetSyncCursor(repoPath, now.Format(time.RFC3339))
	if err != nil {
		return store.SyncState{}, fmt.Errorf("advance repo cursor: %w", err)
	}
	return state, nil
}

func cursorRepoPath(repoPath string) (string, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "", fmt.Errorf("repo path is required")
	}
	return repoPath, nil
}

func cursorArgs(db *store.DB, now time.Time) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}
	if now.IsZero() {
		return fmt.Errorf("time is required")
	}
	return nil
}
