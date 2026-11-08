// Package derive collapses events into per-project, per-day presence.
//
// Grouping is binary: one row per project per local calendar day, not a
// session or a duration. Git is the primary signal. A file-touch only
// supplies the source when that day has no commit.
package derive

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// Source is why a project-day counts as worked.
type Source string

const (
	// SourceCommit means the day has at least one commit. File touches on
	// that day still contribute the file count and sample.
	SourceCommit Source = "commit"
	// SourceFile means the day has file-touch events and no commits.
	SourceFile Source = "file"
)

// DayPresence is one project on one local calendar day.
// CommitHashes is chronological and deduped. Ticket text is not derived here.
type DayPresence struct {
	ProjectSlug  string
	Day          string // YYYY-MM-DD in the location passed to FromEvents
	Worked       bool
	Source       Source
	Branch       string
	CommitHashes []string
	FileCount    int
	FileSample   string
}

type groupKey struct {
	slug string
	day  string
}

type commitBit struct {
	at     time.Time
	hash   string
	branch string
}

type fileBit struct {
	at     time.Time
	index  int
	count  int
	sample string
}

type group struct {
	commits []commitBit
	files   []fileBit
}

// FromEvents groups events into one DayPresence per (project, local date).
//
// loc nil means time.Local, the same day boundary the file scan uses.
// A commit day uses source commit and the branch of the latest kept commit.
// A repeated hash keeps the earliest event, so a later duplicate does not
// move the branch or the commit list. File count is the sum of file-touch
// counts; the sample is the latest file-touch's sample.
//
// Events with an empty project slug are skipped — they are not a project yet.
// Unknown kinds are skipped. The input slice is not modified.
// A zero occurred_at, or a commit with no hash, is an error.
// No groups returns nil, not an empty slice.
func FromEvents(events []store.Event, loc *time.Location) ([]DayPresence, error) {
	if loc == nil {
		loc = time.Local
	}

	groups := make(map[groupKey]*group)
	var keys []groupKey
	for i, event := range events {
		if event.OccurredAt.IsZero() {
			return nil, fmt.Errorf("derive presence: event %d: occurred_at is required", i)
		}
		slug := strings.TrimSpace(event.ProjectSlug)
		if slug == "" {
			continue
		}
		switch event.Kind {
		case store.KindCommit:
			if strings.TrimSpace(event.Hash) == "" {
				return nil, fmt.Errorf("derive presence: event %d: commit hash is required", i)
			}
		case store.KindFileTouch:
		default:
			continue
		}

		key := groupKey{slug: slug, day: event.OccurredAt.In(loc).Format("2006-01-02")}
		g, ok := groups[key]
		if !ok {
			g = &group{}
			groups[key] = g
			keys = append(keys, key)
		}
		g.add(i, event)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].slug != keys[j].slug {
			return keys[i].slug < keys[j].slug
		}
		return keys[i].day < keys[j].day
	})
	if len(keys) == 0 {
		return nil, nil
	}

	rows := make([]DayPresence, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, groups[key].row(key))
	}
	return rows, nil
}

func (g *group) add(index int, event store.Event) {
	switch event.Kind {
	case store.KindCommit:
		g.commits = append(g.commits, commitBit{
			at:     event.OccurredAt,
			hash:   event.Hash,
			branch: event.Branch,
		})
	case store.KindFileTouch:
		g.files = append(g.files, fileBit{
			at:     event.OccurredAt,
			index:  index,
			count:  event.FileCount,
			sample: event.FileSample,
		})
	}
}

func (g *group) row(key groupKey) DayPresence {
	hashes, branch := keptCommits(g.commits)
	count, sample := filePresence(g.files)
	source := SourceFile
	if len(hashes) > 0 {
		source = SourceCommit
	}
	return DayPresence{
		ProjectSlug:  key.slug,
		Day:          key.day,
		Worked:       true,
		Source:       source,
		Branch:       branch,
		CommitHashes: hashes,
		FileCount:    count,
		FileSample:   sample,
	}
}

// keptCommits returns unique hashes in time order, and the branch of the
// latest kept commit. The earliest event wins for a repeated hash.
func keptCommits(commits []commitBit) ([]string, string) {
	if len(commits) == 0 {
		return nil, ""
	}
	ordered := append([]commitBit(nil), commits...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].at.Equal(ordered[j].at) {
			return ordered[i].at.Before(ordered[j].at)
		}
		return ordered[i].hash < ordered[j].hash
	})

	seen := make(map[string]struct{}, len(ordered))
	hashes := make([]string, 0, len(ordered))
	var branch string
	for _, commit := range ordered {
		if _, ok := seen[commit.hash]; ok {
			continue
		}
		seen[commit.hash] = struct{}{}
		hashes = append(hashes, commit.hash)
		branch = commit.branch
	}
	return hashes, branch
}

func filePresence(files []fileBit) (int, string) {
	if len(files) == 0 {
		return 0, ""
	}
	latest := files[0]
	count := 0
	for _, file := range files {
		count += file.count
		if file.at.After(latest.at) || (file.at.Equal(latest.at) && file.index > latest.index) {
			latest = file
		}
	}
	return count, latest.sample
}
