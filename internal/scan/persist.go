package scan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hackastak/hackatrak/internal/store"
)

// PersistedCommits is the outcome of writing commits as events.
// Inserted are the new rows, in input order. Skipped counts hashes that
// were already stored, or repeated later in the same batch.
type PersistedCommits struct {
	Inserted []store.Event
	Skipped  int
}

// PersistCommits writes commits as commit events, deduped by hash.
//
// A hash already in the store is skipped, not an error, so a re-scan does
// not double-count. Dedup is global: the same hash from another repo is
// skipped too. A hash repeated in commits keeps the first occurrence.
// The input slice is not modified. An empty commits slice is a no-op.
//
// projectSlug may be empty; project detection fills it later.
// A store error may leave earlier commits in the batch already inserted.
// Retrying is safe because those hashes are then skipped.
func PersistCommits(db *store.DB, repoPath, projectSlug string, commits []Commit) (PersistedCommits, error) {
	if db == nil {
		return PersistedCommits{}, fmt.Errorf("persist commits: database is required")
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return PersistedCommits{}, fmt.Errorf("persist commits: repo path is required")
	}
	if err := validateCommits(commits); err != nil {
		return PersistedCommits{}, fmt.Errorf("persist commits: %w", err)
	}

	var result PersistedCommits
	seen := make(map[string]struct{}, len(commits))
	for _, commit := range commits {
		if _, ok := seen[commit.Hash]; ok {
			result.Skipped++
			continue
		}
		seen[commit.Hash] = struct{}{}

		inserted, err := db.InsertEvent(store.Event{
			Kind:        store.KindCommit,
			Hash:        commit.Hash,
			RepoPath:    repoPath,
			ProjectSlug: projectSlug,
			OccurredAt:  commit.Time,
			Branch:      commit.Branch,
			Subject:     commit.Subject,
			Body:        commit.Body,
		})
		if err != nil {
			if errors.Is(err, store.ErrDuplicate) {
				result.Skipped++
				continue
			}
			return PersistedCommits{}, fmt.Errorf("persist commits: %w", err)
		}
		result.Inserted = append(result.Inserted, inserted)
	}
	return result, nil
}

func validateCommits(commits []Commit) error {
	for i, commit := range commits {
		if strings.TrimSpace(commit.Hash) == "" {
			return fmt.Errorf("commit %d: hash is required", i)
		}
		if commit.Time.IsZero() {
			return fmt.Errorf("commit %d: time is required", i)
		}
	}
	return nil
}
