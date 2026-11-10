package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/derive"
	"github.com/hackastak/hackatrak/internal/scan"
	"github.com/hackastak/hackatrak/internal/store"
	"github.com/hackastak/hackatrak/internal/summarize"
	"github.com/hackastak/hackatrak/internal/vault"
)

// noteBullet is one line ready to splice or spool. Text has no marker;
// the vault writer appends <!-- ht:<slug> -->. Day is YYYY-MM-DD.
type noteBullet struct {
	Day    string
	Bucket string
	Slug   string
	Ticket string
	Text   string
}

// writeOwnBuckets splices the writer role's note.
// Today's own-bucket bullets are merged with current-week spool days for
// the SAP heading. A spooler must not call this. An idle sync does not
// create a missing week file. dry-run never reaches this function.
func writeOwnBuckets(cfg config.Config, db *store.DB, now time.Time, logw io.Writer) (int, error) {
	if cfg.Role != config.RoleWriter {
		return 0, nil
	}
	logger := log.New(logw, "", 0)
	own, err := ownBullets(cfg, db, now, logger)
	if err != nil {
		return 0, err
	}
	remote, err := spoolBullets(cfg, now, logger)
	if err != nil {
		return 0, err
	}
	bullets := mergeNoteBullets(own, remote)
	if len(bullets) == 0 {
		return 0, nil
	}
	path, err := vault.EnsureWeekNote(cfg.VaultPath, now)
	if err != nil {
		return 0, fmt.Errorf("sync: %w", err)
	}
	wrote := 0
	for _, bullet := range bullets {
		when, err := bulletTime(bullet.Day, now)
		if err != nil {
			return 0, err
		}
		before, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("sync: read note: %w", err)
		}
		err = vault.WriteAtomic(path, func(note string) (string, error) {
			return vault.Splice(note, bullet.Bucket, when, vault.Bullet{
				Slug: bullet.Slug,
				Text: bullet.Text,
			}, vault.SpliceOptions{
				Missing: vault.CreateMissing,
				Log:     logger,
			})
		})
		if err != nil {
			return 0, fmt.Errorf("sync: write %s: %w", bullet.Slug, err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("sync: read note: %w", err)
		}
		if !bytes.Equal(before, after) {
			wrote++
		}
	}
	return wrote, nil
}

func ownBullets(cfg config.Config, db *store.DB, now time.Time, logger *log.Logger) ([]noteBullet, error) {
	return bulletsFor(cfg, db, now, logger, func(at time.Time) bool {
		return sameLocalDay(at, now)
	})
}

func weekBullets(cfg config.Config, db *store.DB, now time.Time, logger *log.Logger) ([]noteBullet, error) {
	year, week := now.ISOWeek()
	return bulletsFor(cfg, db, now, logger, func(at time.Time) bool {
		gotYear, gotWeek := at.In(now.Location()).ISOWeek()
		return gotYear == year && gotWeek == week
	})
}

func bulletsFor(cfg config.Config, db *store.DB, now time.Time, logger *log.Logger, keep func(time.Time) bool) ([]noteBullet, error) {
	summarizer, err := summarize.FromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	paths, err := activityPaths(cfg)
	if err != nil {
		return nil, err
	}
	roots := rootPaths(cfg)
	own := ownBuckets(cfg)
	var bullets []noteBullet
	for _, path := range paths {
		events, err := db.EventsByRepo(path)
		if err != nil {
			return nil, fmt.Errorf("sync: %w", err)
		}
		selected := eventsKept(events, keep)
		if len(selected) == 0 {
			continue
		}
		project, err := scan.ResolveProject(path, roots)
		if errors.Is(err, scan.ErrNoProject) {
			if logger != nil {
				logger.Printf("sync: skip %s: no project", path)
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("sync: project %s: %w", path, err)
		}
		bucket, err := scan.ResolveBucket(path, cfg.Roots)
		if err != nil {
			return nil, fmt.Errorf("sync: bucket %s: %w", path, err)
		}
		if _, ok := own[bucket]; !ok {
			continue
		}
		filled := withSlug(selected, project.Slug)
		rows, err := derive.FromEvents(filled, now.Location())
		if err != nil {
			return nil, fmt.Errorf("sync: %w", err)
		}
		for _, row := range rows {
			if row.Day != now.Format("2006-01-02") || !row.Worked {
				continue
			}
			summary, err := summarizer.Summarize(summaryDay(row, filled))
			if err != nil {
				return nil, fmt.Errorf("sync: summarize %s: %w", project.Slug, err)
			}
			if err := upsertPresence(db, row, summary.Ticket); err != nil {
				return nil, fmt.Errorf("sync: %w", err)
			}
			text := formatBullet(project.Slug, summary)
			if text == "" {
				continue
			}
			bullets = append(bullets, noteBullet{
				Day:    row.Day,
				Bucket: bucket,
				Slug:   project.Slug,
				Ticket: summary.Ticket,
				Text:   text,
			})
		}
	}
	sort.Slice(bullets, func(i, j int) bool {
		if bullets[i].Day != bullets[j].Day {
			return bullets[i].Day < bullets[j].Day
		}
		if bullets[i].Bucket != bullets[j].Bucket {
			return bullets[i].Bucket < bullets[j].Bucket
		}
		return bullets[i].Slug < bullets[j].Slug
	})
	return bullets, nil
}

func activityPaths(cfg config.Config) ([]string, error) {
	seen := make(map[string]struct{})
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	for _, root := range cfg.Roots {
		add(root.Path)
		repos, err := scan.DiscoverRepos(root.Path)
		if err != nil {
			return nil, fmt.Errorf("sync: %w", err)
		}
		for _, repo := range repos {
			add(repo)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func ownBuckets(cfg config.Config) map[string]struct{} {
	own := make(map[string]struct{}, len(cfg.Roots))
	for _, root := range cfg.Roots {
		bucket := strings.TrimSpace(root.Bucket)
		if bucket == "" {
			continue
		}
		own[bucket] = struct{}{}
	}
	return own
}

func eventsKept(events []store.Event, keep func(time.Time) bool) []store.Event {
	kept := make([]store.Event, 0, len(events))
	for _, event := range events {
		if keep != nil && !keep(event.OccurredAt) {
			continue
		}
		kept = append(kept, event)
	}
	return kept
}

func bulletTime(day string, now time.Time) (time.Time, error) {
	if strings.TrimSpace(day) == "" {
		return now, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", day, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("sync: day %q: %w", day, err)
	}
	return parsed, nil
}

func mergeNoteBullets(own, remote []noteBullet) []noteBullet {
	ownKey := make(map[string]struct{}, len(own))
	for _, bullet := range own {
		ownKey[bulletKey(bullet)] = struct{}{}
	}
	merged := make([]noteBullet, 0, len(own)+len(remote))
	for _, bullet := range remote {
		if _, ok := ownKey[bulletKey(bullet)]; ok {
			continue
		}
		merged = append(merged, bullet)
	}
	return append(merged, own...)
}

func bulletKey(bullet noteBullet) string {
	return bullet.Day + "\x00" + bullet.Bucket + "\x00" + bullet.Slug
}

func withSlug(events []store.Event, slug string) []store.Event {
	filled := make([]store.Event, len(events))
	for i, event := range events {
		event.ProjectSlug = slug
		filled[i] = event
	}
	return filled
}

func summaryDay(row derive.DayPresence, events []store.Event) summarize.Day {
	byHash := make(map[string]store.Event, len(events))
	for _, event := range events {
		if event.Kind == store.KindCommit && event.Hash != "" {
			byHash[event.Hash] = event
		}
	}
	day := summarize.Day{
		Branch:     row.Branch,
		FileCount:  row.FileCount,
		FileSample: row.FileSample,
	}
	for _, hash := range row.CommitHashes {
		event, ok := byHash[hash]
		if !ok {
			continue
		}
		day.Commits = append(day.Commits, summarize.Commit{
			Subject: event.Subject,
			Body:    event.Body,
			Branch:  event.Branch,
		})
	}
	return day
}

func upsertPresence(db *store.DB, row derive.DayPresence, ticket string) error {
	source := store.PresenceFile
	if row.Source == derive.SourceCommit {
		source = store.PresenceCommit
	}
	_, err := db.UpsertDayPresence(store.DayPresence{
		ProjectSlug:  row.ProjectSlug,
		Day:          row.Day,
		Worked:       row.Worked,
		Source:       source,
		Branch:       row.Branch,
		Ticket:       ticket,
		CommitHashes: row.CommitHashes,
		FileCount:    row.FileCount,
		FileSample:   row.FileSample,
	})
	return err
}

func formatBullet(slug string, summary summarize.Summary) string {
	line := strings.TrimSpace(summary.Line)
	if slug == "" || line == "" {
		return ""
	}
	if summary.Ticket != "" {
		return fmt.Sprintf("- [[%s]] (%s): %s", slug, summary.Ticket, line)
	}
	return fmt.Sprintf("- [[%s]]: %s", slug, line)
}

func sameLocalDay(at, now time.Time) bool {
	loc := now.Location()
	ay, am, ad := at.In(loc).Date()
	ny, nm, nd := now.In(loc).Date()
	return ay == ny && am == nm && ad == nd
}
