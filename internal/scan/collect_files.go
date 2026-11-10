package scan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// FileCollect is the file-mtime half of a sync collect pass.
// Touches are file-only presence: a repo that already has a commit today is
// omitted, so the git signal is not counted again. Inserted, Updated, and
// Removed are zero on a dry run, which does not write.
type FileCollect struct {
	Touches  []FileTouch
	Inserted int
	Updated  int
	Removed  int
}

// CollectFiles scans roots for files changed since local midnight and emits
// one file-touch presence event per walk root that has any.
//
// Files inside a discovered git repo are attributed to that repo, not to the
// configured root, so the same file is not recorded twice. A repo with a
// commit on today's local calendar day — already stored, or named in
// committedRepos from this pass — does not get a file-touch. Git is the
// presence signal there. A real run also removes a same-day file-touch for
// that repo, so an earlier file-only scan is not left beside the commit.
// Other repos in the same pass still get a file-touch. dryRun reports the
// file-only touches and leaves events unchanged.
//
// excludeGlobs are extra patterns on top of the built-in mtime exclusions.
// now must be non-zero. project detection fills the event slug later.
func CollectFiles(db *store.DB, roots, excludeGlobs []string, now time.Time, dryRun bool, committedRepos []string) (FileCollect, error) {
	return collectFiles(db, roots, excludeGlobs, now, time.Time{}, dryRun, committedRepos)
}

// CollectFilesUntil is CollectFiles limited to mtimes before until.
// until is exclusive and must be after local midnight of now.
func CollectFilesUntil(db *store.DB, roots, excludeGlobs []string, now, until time.Time, dryRun bool, committedRepos []string) (FileCollect, error) {
	if until.IsZero() || !until.After(LocalMidnight(now)) {
		return FileCollect{}, fmt.Errorf("collect files: until must be after the day's start")
	}
	return collectFiles(db, roots, excludeGlobs, now, until, dryRun, committedRepos)
}

func collectFiles(db *store.DB, roots, excludeGlobs []string, now, until time.Time, dryRun bool, committedRepos []string) (FileCollect, error) {
	if db == nil {
		return FileCollect{}, fmt.Errorf("collect files: database is required")
	}
	roots, err := collectRoots(roots)
	if err != nil {
		return FileCollect{}, fmt.Errorf("collect files: %w", err)
	}
	if now.IsZero() {
		return FileCollect{}, fmt.Errorf("collect files: time is required")
	}

	repos, err := discoverAll(roots)
	if err != nil {
		return FileCollect{}, fmt.Errorf("collect files: %w", err)
	}
	covered, err := reposCoveredByGit(db, repos, committedRepos, now)
	if err != nil {
		return FileCollect{}, fmt.Errorf("collect files: %w", err)
	}

	var result FileCollect
	if !dryRun {
		removed, err := dropCoveredFileTouches(db, covered, now)
		if err != nil {
			return FileCollect{}, fmt.Errorf("collect files: %w", err)
		}
		result.Removed = removed
	}

	files, err := filesInWindow(roots, LocalMidnight(now), until, excludeGlobs)
	if err != nil {
		return FileCollect{}, fmt.Errorf("collect files: %w", err)
	}
	touches, err := FileTouches(attributeFiles(files, repos, covered))
	if err != nil {
		return FileCollect{}, fmt.Errorf("collect files: %w", err)
	}
	result.Touches = touches
	if dryRun || len(touches) == 0 {
		return result, nil
	}

	for _, touch := range touches {
		_, updated, err := persistFileTouch(db, "", touch)
		if err != nil {
			return FileCollect{}, fmt.Errorf("collect files: %w", err)
		}
		if updated {
			result.Updated++
			continue
		}
		result.Inserted++
	}
	return result, nil
}

func filesInWindow(roots []string, since, until time.Time, excludeGlobs []string) ([]TouchedFile, error) {
	if until.IsZero() {
		return FilesSince(roots, since, excludeGlobs...)
	}
	return FilesBetween(roots, since, until, excludeGlobs...)
}

func discoverAll(roots []string) ([]string, error) {
	seen := make(map[string]struct{})
	var repos []string
	for _, root := range roots {
		found, err := DiscoverRepos(root)
		if err != nil {
			return nil, err
		}
		for _, repo := range found {
			if _, ok := seen[repo]; ok {
				continue
			}
			seen[repo] = struct{}{}
			repos = append(repos, repo)
		}
	}
	sort.Strings(repos)
	return repos, nil
}

func reposCoveredByGit(db *store.DB, repos, committed []string, now time.Time) (map[string]struct{}, error) {
	covered := commitSet(committed)
	for _, repo := range repos {
		if _, ok := covered[repo]; ok {
			continue
		}
		events, err := db.EventsByRepo(repo)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if event.Kind == store.KindCommit && sameDay(event.OccurredAt, now) {
				covered[repo] = struct{}{}
				break
			}
		}
	}
	return covered, nil
}

func commitSet(repos []string) map[string]struct{} {
	set := make(map[string]struct{}, len(repos))
	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			continue
		}
		set[repo] = struct{}{}
		abs, err := filepath.Abs(repo)
		if err != nil {
			continue
		}
		set[abs] = struct{}{}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			continue
		}
		set[resolved] = struct{}{}
	}
	return set
}

func dropCoveredFileTouches(db *store.DB, covered map[string]struct{}, now time.Time) (int, error) {
	removed := 0
	repos := make([]string, 0, len(covered))
	for repo := range covered {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	for _, repo := range repos {
		events, err := db.EventsByRepo(repo)
		if err != nil {
			return removed, err
		}
		for _, event := range events {
			if event.Kind != store.KindFileTouch || !sameDay(event.OccurredAt, now) {
				continue
			}
			if err := db.DeleteFileTouch(event.ID); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

// attributeFiles copies files, points each one at its enclosing repo, and
// drops files whose repo already has a commit today. Files outside any repo
// keep the walk root FilesSince recorded.
func attributeFiles(files []TouchedFile, repos []string, covered map[string]struct{}) []TouchedFile {
	attributed := make([]TouchedFile, 0, len(files))
	for _, file := range files {
		if repo, ok := repoContaining(file.Path, repos); ok {
			file.Root = repo
		}
		if _, skip := covered[file.Root]; skip {
			continue
		}
		attributed = append(attributed, file)
	}
	return attributed
}

func repoContaining(path string, repos []string) (string, bool) {
	var match string
	for _, repo := range repos {
		prefix := repo + string(filepath.Separator)
		if strings.HasPrefix(path, prefix) && len(repo) > len(match) {
			match = repo
		}
	}
	return match, match != ""
}

func sameDay(a, b time.Time) bool {
	loc := b.Location()
	ay, am, ad := a.In(loc).Date()
	by, bm, bd := b.In(loc).Date()
	return ay == by && am == bm && ad == bd
}
