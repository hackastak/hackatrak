package main

import (
	"bytes"
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

// noteBullet is one own-bucket line ready to splice. Text has no marker;
// the vault writer appends <!-- ht:<slug> -->.
type noteBullet struct {
	Bucket string
	Slug   string
	Text   string
}

// writeOwnBuckets splices today's bullets for the writer role.
// A spooler returns without reading or creating the weekly note. Only
// buckets named by this config's roots are written. An idle day does not
// create a missing week file. dry-run never reaches this function.
func writeOwnBuckets(cfg config.Config, db *store.DB, now time.Time, logw io.Writer) (int, error) {
	if cfg.Role != config.RoleWriter {
		return 0, nil
	}
	bullets, err := ownBullets(cfg, db, now)
	if err != nil {
		return 0, err
	}
	if len(bullets) == 0 {
		return 0, nil
	}
	path, err := vault.EnsureWeekNote(cfg.VaultPath, now)
	if err != nil {
		return 0, fmt.Errorf("sync: %w", err)
	}
	logger := log.New(logw, "", 0)
	wrote := 0
	for _, bullet := range bullets {
		before, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("sync: read note: %w", err)
		}
		err = vault.WriteAtomic(path, func(note string) (string, error) {
			return vault.Splice(note, bullet.Bucket, now, vault.Bullet{
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

func ownBullets(cfg config.Config, db *store.DB, now time.Time) ([]noteBullet, error) {
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
		today := eventsOnDay(events, now)
		if len(today) == 0 {
			continue
		}
		project, err := scan.ResolveProject(path, roots)
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
		filled := withSlug(today, project.Slug)
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
				Bucket: bucket,
				Slug:   project.Slug,
				Text:   text,
			})
		}
	}
	sort.Slice(bullets, func(i, j int) bool {
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

func eventsOnDay(events []store.Event, now time.Time) []store.Event {
	today := make([]store.Event, 0, len(events))
	for _, event := range events {
		if !sameLocalDay(event.OccurredAt, now) {
			continue
		}
		today = append(today, event)
	}
	return today
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
