package scan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// fileSampleLimit is how many file names a presence event keeps.
// The count still includes every touched file.
const fileSampleLimit = 3

// FileTouch is one walk root's mtime presence: how many files changed, and
// a small sample of their file names.
type FileTouch struct {
	Root       string
	Count      int
	Sample     string
	OccurredAt time.Time
}

// PersistedFileTouches is the outcome of writing file-touch events.
// Inserted are new rows. Updated are same-local-day rows replaced in place.
type PersistedFileTouches struct {
	Inserted []store.Event
	Updated  []store.Event
}

// FileTouches groups touched files by walk root.
//
// Each root becomes one presence value: Count is every file, Sample is at
// most three base names in path order (a repeated name is listed once), and
// OccurredAt is the latest mtime. Roots are returned in path order. The
// input slice is not modified. An empty input returns nil.
func FileTouches(files []TouchedFile) ([]FileTouch, error) {
	if len(files) == 0 {
		return nil, nil
	}
	grouped := make(map[string][]TouchedFile)
	for i, file := range files {
		root := strings.TrimSpace(file.Root)
		if root == "" {
			return nil, fmt.Errorf("file touches: file %d: root is required", i)
		}
		if strings.TrimSpace(file.Path) == "" {
			return nil, fmt.Errorf("file touches: file %d: path is required", i)
		}
		if file.MTime.IsZero() {
			return nil, fmt.Errorf("file touches: file %d: mtime is required", i)
		}
		grouped[root] = append(grouped[root], file)
	}

	roots := make([]string, 0, len(grouped))
	for root := range grouped {
		roots = append(roots, root)
	}
	sort.Strings(roots)

	touches := make([]FileTouch, 0, len(roots))
	for _, root := range roots {
		touches = append(touches, fileTouch(root, grouped[root]))
	}
	return touches, nil
}

// PersistFileTouches emits one file-touch event per root that has files.
//
// The event stores the changed-file count and a small filename sample.
// projectSlug may be empty; project detection fills it later. A root that
// already has a file-touch on the same local calendar day is updated in
// place, so a later scan the same day replaces the count and sample instead
// of adding another presence row. An empty slug on that update keeps the
// slug already stored. A file-touch from another local day, and any commit
// event, is left alone. The input slice is not modified.
//
// An empty files slice is a no-op. Invalid files are rejected before any
// write. A store error may leave earlier roots in the batch already written.
// Retrying is safe: those roots are then updated, not inserted again.
func PersistFileTouches(db *store.DB, projectSlug string, files []TouchedFile) (PersistedFileTouches, error) {
	if db == nil {
		return PersistedFileTouches{}, fmt.Errorf("persist file touches: database is required")
	}
	touches, err := FileTouches(files)
	if err != nil {
		return PersistedFileTouches{}, fmt.Errorf("persist file touches: %w", err)
	}

	var result PersistedFileTouches
	for _, touch := range touches {
		persisted, updated, err := persistFileTouch(db, projectSlug, touch)
		if err != nil {
			return PersistedFileTouches{}, fmt.Errorf("persist file touches: %w", err)
		}
		if updated {
			result.Updated = append(result.Updated, persisted)
			continue
		}
		result.Inserted = append(result.Inserted, persisted)
	}
	return result, nil
}

func fileTouch(root string, files []TouchedFile) FileTouch {
	sorted := append([]TouchedFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	var latest time.Time
	names := make([]string, 0, fileSampleLimit)
	seen := make(map[string]struct{})
	for _, file := range sorted {
		if file.MTime.After(latest) {
			latest = file.MTime
		}
		name := filepath.Base(file.Path)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if len(names) == fileSampleLimit {
			continue
		}
		names = append(names, name)
	}
	return FileTouch{
		Root:       root,
		Count:      len(sorted),
		Sample:     strings.Join(names, ", "),
		OccurredAt: latest,
	}
}

func persistFileTouch(db *store.DB, projectSlug string, touch FileTouch) (store.Event, bool, error) {
	existing, found, err := fileTouchOnDay(db, touch.Root, touch.OccurredAt)
	if err != nil {
		return store.Event{}, false, err
	}
	if found {
		slug := projectSlug
		if strings.TrimSpace(slug) == "" {
			slug = existing.ProjectSlug
		}
		updated, err := db.UpdateFileTouch(existing.ID, touch.OccurredAt, touch.Count, touch.Sample, slug)
		if err != nil {
			return store.Event{}, false, err
		}
		return updated, true, nil
	}

	inserted, err := db.InsertEvent(store.Event{
		Kind:        store.KindFileTouch,
		RepoPath:    touch.Root,
		ProjectSlug: projectSlug,
		OccurredAt:  touch.OccurredAt,
		FileCount:   touch.Count,
		FileSample:  touch.Sample,
	})
	if err != nil {
		return store.Event{}, false, err
	}
	return inserted, false, nil
}

func fileTouchOnDay(db *store.DB, root string, when time.Time) (store.Event, bool, error) {
	events, err := db.EventsByRepo(root)
	if err != nil {
		return store.Event{}, false, err
	}
	year, month, day := when.In(time.Local).Date()
	var found store.Event
	ok := false
	for _, event := range events {
		if event.Kind != store.KindFileTouch {
			continue
		}
		ey, em, ed := event.OccurredAt.In(time.Local).Date()
		if ey != year || em != month || ed != day {
			continue
		}
		if ok && (event.OccurredAt.Before(found.OccurredAt) || (event.OccurredAt.Equal(found.OccurredAt) && event.ID <= found.ID)) {
			continue
		}
		found = event
		ok = true
	}
	return found, ok, nil
}
