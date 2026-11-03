package scan

import (
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

			found, err := collectRepo(db, repo, authors, now, dryRun)
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

func collectRepo(db *store.DB, repo string, authors []string, now time.Time, dryRun bool) (GitCollect, error) {
	since, err := RepoSince(db, repo, now)
	if err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	commits, err := CommitsSince(repo, since, authors)
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
	if _, err := AdvanceRepoCursor(db, repo, now); err != nil {
		return GitCollect{}, fmt.Errorf("collect git: %w", err)
	}
	result.Inserted = len(persisted.Inserted)
	result.Skipped = persisted.Skipped
	return result, nil
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
