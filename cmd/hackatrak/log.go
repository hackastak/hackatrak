package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/scan"
	"github.com/hackastak/hackatrak/internal/summarize"
	"github.com/hackastak/hackatrak/internal/vault"
)

// logEntryRe parses `project: did X` and `project (TICKET): did X`.
// The project is the wikilink slug. Ticket is optional and has no parentheses
// in the capture. Text is the summary clause, not a bullet.
var logEntryRe = regexp.MustCompile(`^(.+?)(?:\s+\(([^)]+)\))?\s*:\s*(\S.*)$`)

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   `log "project: did X"`,
		Short: "Splice a manual project bullet for today",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(cmd, args[0])
		},
	}
}

func runLog(cmd *cobra.Command, entry string) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	if state.cfg.Role != config.RoleWriter {
		return fmt.Errorf("log: spooler role does not write the weekly note")
	}
	parsed, err := parseLogEntry(entry)
	if err != nil {
		return err
	}
	bucket, err := loggedBucket(state.cfg, parsed.slug)
	if err != nil {
		return err
	}
	text := formatBullet(parsed.slug, summarize.Summary{Line: parsed.text, Ticket: parsed.ticket})
	if text == "" {
		return fmt.Errorf("log: entry is empty")
	}
	now := time.Now()
	path, err := vault.EnsureWeekNote(state.cfg.VaultPath, now)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	err = vault.WriteAtomic(path, func(note string) (string, error) {
		return vault.Splice(note, bucket, now, vault.Bullet{
			Slug: parsed.slug,
			Text: text,
		}, vault.SpliceOptions{
			Missing: vault.CreateMissing,
			Log:     log.New(cmd.ErrOrStderr(), "", 0),
		})
	})
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "log: "+text)
	return err
}

type logEntry struct {
	slug   string
	ticket string
	text   string
}

func parseLogEntry(entry string) (logEntry, error) {
	match := logEntryRe.FindStringSubmatch(strings.TrimSpace(entry))
	if match == nil {
		return logEntry{}, fmt.Errorf("log: entry must be \"project: did X\"")
	}
	parsed := logEntry{
		slug:   strings.TrimSpace(match[1]),
		ticket: strings.TrimSpace(match[2]),
		text:   strings.TrimSpace(match[3]),
	}
	if parsed.slug == "" || parsed.text == "" {
		return logEntry{}, fmt.Errorf("log: entry must be \"project: did X\"")
	}
	return parsed, nil
}

func loggedBucket(cfg config.Config, slug string) (string, error) {
	var buckets []string
	for _, root := range cfg.Roots {
		paths, err := projectPaths(root.Path)
		if err != nil {
			return "", fmt.Errorf("log: %w", err)
		}
		for _, path := range paths {
			if filepath.Base(path) != slug {
				continue
			}
			bucket, err := scan.ResolveBucket(path, cfg.Roots)
			if err != nil {
				return "", fmt.Errorf("log: %w", err)
			}
			buckets = append(buckets, bucket)
		}
	}
	if len(buckets) == 0 {
		return "", fmt.Errorf("log: project %q is not under a configured root", slug)
	}
	for _, bucket := range buckets[1:] {
		if bucket != buckets[0] {
			return "", fmt.Errorf("log: project %q matches more than one bucket", slug)
		}
	}
	return buckets[0], nil
}

func projectPaths(root string) ([]string, error) {
	paths := []string{root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", root, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name == ".git" {
			continue
		}
		paths = append(paths, filepath.Join(root, name))
	}
	repos, err := scan.DiscoverRepos(root)
	if err != nil {
		return nil, err
	}
	return append(paths, repos...), nil
}
