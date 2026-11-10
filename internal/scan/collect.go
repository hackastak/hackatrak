package scan

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// DetectedCommit is one commit found while collecting a root.
// RepoPath is the absolute path DiscoverRepos returns, which is also the
// sync_state key.
type DetectedCommit struct {
	RepoPath string
	Commit
}

// GitCollect is the git half of a sync collect pass.
// Commits are the ones in the scan window, oldest first within each repo.
// Inserted and Skipped are zero on a dry run, which does not write.
type GitCollect struct {
	Commits  []DetectedCommit
	Inserted int
	Skipped  int
}

// CollectGit scans roots for commits by authors.
//
// Each repo is read from its sync_state cursor, or from local midnight when
// no cursor exists. dryRun reports that window and leaves events and cursors
// unchanged. A real run persists new commits, then advances that repo's cursor
// to now — including a repo with no matching commits, so the next pass does
// not rescan the same window. now must be non-zero.
//
// Roots are walked in order. A repo found under more than one root is scanned
// once. Author emails are required even when no repos are found, so a missing
// filter fails before any cursor moves. project detection fills the event
// slug later; this pass stores commits with an empty slug.
func CollectGit(db *store.DB, roots, authors []string, now time.Time, dryRun bool) (GitCollect, error) {
	return collectGit(db, roots, authors, now, time.Time{}, dryRun)
}

// CollectGitUntil is CollectGit limited to commits before until.
// until is exclusive. A stored cursor is not moved earlier than it already is.
// until must be after local midnight of now.
func CollectGitUntil(db *store.DB, roots, authors []string, now, until time.Time, dryRun bool) (GitCollect, error) {
	if until.IsZero() || !until.After(LocalMidnight(now)) {
		return GitCollect{}, fmt.Errorf("collect git: until must be after the day's start")
	}
	return collectGit(db, roots, authors, now, until, dryRun)
}

func collectGit(db *store.DB, roots, authors []string, now, until time.Time, dryRun bool) (GitCollect, error) {
	roots, err := collectRoots(roots)
	if err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	if _, err := authorEmails(authors); err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	if err := cursorArgs(db, now); err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}

	var result GitCollect
	seen := make(map[string]struct{})
	for _, root := range roots {
		repos, err := DiscoverRepos(root)
		if err != nil {
			return GitCollect{}, fmt.Errorf("collect git: %w", err)
		}
		for _, repo := range repos {
			if _, ok := seen[repo]; ok {
				continue
			}
			seen[repo] = struct{}{}

			found, err := collectRepo(db, repo, authors, now, until, dryRun)
			if err != nil {
				return GitCollect{}, err
			}
			result.Commits = append(result.Commits, found.Commits...)
			result.Inserted += found.Inserted
			result.Skipped += found.Skipped
		}
	}
	return result, nil
}

func collectRepo(db *store.DB, repo string, authors []string, now, until time.Time, dryRun bool) (GitCollect, error) {
	since, bounded, err := repoWindow(db, repo, now, until)
	if err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	if bounded && since == "" {
		return GitCollect{}, nil
	}
	var commits []Commit
	if until.IsZero() {
		commits, err = CommitsSince(repo, since, authors)
	} else {
		commits, err = commitsSince(repo, since, until.Format(time.RFC3339), authors)
	}
	if err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}

	var result GitCollect
	for _, commit := range commits {
		result.Commits = append(result.Commits, DetectedCommit{RepoPath: repo, Commit: commit})
	}
	if dryRun {
		return result, nil
	}

	persisted, err := PersistCommits(db, repo, "", commits)
	if err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	if until.IsZero() {
		if _, err := AdvanceRepoCursor(db, repo, now); err != nil {
			return GitCollect{}, fmt.Errorf("collect git: %w", err)
		}
	} else if err := advanceIfForward(db, repo, now); err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	result.Inserted = len(persisted.Inserted)
	result.Skipped = persisted.Skipped
	return result, nil
}

// repoWindow returns the git --since value. A bounded window that the cursor
// has already passed returns an empty since so the caller skips the repo.
func repoWindow(db *store.DB, repo string, now, until time.Time) (string, bool, error) {
	since, err := RepoSince(db, repo, now)
	if err != nil {
		return "", false, err
	}
	if until.IsZero() {
		return since, false, nil
	}
	start := LocalMidnight(now)
	sinceTime, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return "", false, fmt.Errorf("repo since %s: %w", repo, err)
	}
	if !sinceTime.Before(until) {
		return "", true, nil
	}
	if sinceTime.Before(start) {
		since = start.Format(time.RFC3339)
	}
	return since, true, nil
}

func advanceIfForward(db *store.DB, repo string, now time.Time) error {
	state, err := db.SyncState(repo)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err == nil {
		cursor, parseErr := time.Parse(time.RFC3339, state.Cursor)
		if parseErr == nil && !now.After(cursor) {
			return nil
		}
	}
	_, err = AdvanceRepoCursor(db, repo, now)
	return err
}

func collectRoots(roots []string) ([]string, error) {
	if len(roots) == 0 {
		return nil, fmt.Errorf("at least one root is required")
	}
	cleaned := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			return nil, fmt.Errorf("root path is required")
		}
		cleaned = append(cleaned, root)
	}
	return cleaned, nil
}
