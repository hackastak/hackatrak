package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/scan"
	"github.com/hackastak/hackatrak/internal/store"
)

func newSyncCmd() *cobra.Command {
	var dryRun bool
	var verbose bool
	var date string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Scan configured roots and record work",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSync(cmd, dryRun, verbose, date)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print detected commits and file presence without recording them")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log the day, mode, and scan counts to stderr")
	cmd.Flags().StringVar(&date, "date", "", "limit a dry-run to one local day (YYYY-MM-DD)")
	return cmd
}

// runSync collects git commits and file-mtime presence. A repo with a commit
// today is not also recorded as a file-touch. The writer role then splices
// today's own bullets and any current-week spool days into the note. A
// spooler writes only its spool file. --dry-run stops at printing.
func runSync(cmd *cobra.Command, dryRun, verbose bool, date string) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	when, err := resolveSyncWhen(date, dryRun)
	if err != nil {
		return err
	}
	logSync(cmd, verbose, "day %s", when.now.Format("2006-01-02"))
	if dryRun {
		logSync(cmd, verbose, "mode dry-run")
	} else {
		logSync(cmd, verbose, "mode write")
	}
	collected, err := collectSync(state.cfg, state.db, when, dryRun)
	if err != nil {
		return err
	}
	logSync(cmd, verbose, "detected %d commits, %d file touches", len(collected.git.Commits), len(collected.files.Touches))
	if dryRun {
		return printCollect(cmd.OutOrStdout(), collected, true, 0)
	}
	wrote, err := publishSync(state.cfg, state.db, when.now, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	return printCollect(cmd.OutOrStdout(), collected, false, wrote)
}

type syncWhen struct {
	now   time.Time
	until time.Time
}

func resolveSyncWhen(date string, dryRun bool) (syncWhen, error) {
	now := time.Now()
	date = strings.TrimSpace(date)
	if date == "" {
		return syncWhen{now: now}, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return syncWhen{}, usagef("sync: --date must be YYYY-MM-DD")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if parsed.After(today) {
		return syncWhen{}, usagef("sync: --date %s is in the future", date)
	}
	if parsed.Equal(today) {
		return syncWhen{now: now}, nil
	}
	if !dryRun {
		return syncWhen{}, usagef("sync: --date before today requires --dry-run")
	}
	return syncWhen{
		now:   parsed.Add(24*time.Hour - time.Second),
		until: parsed.Add(24 * time.Hour),
	}, nil
}

func logSync(cmd *cobra.Command, verbose bool, format string, args ...any) {
	if !verbose || cmd == nil {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "sync: "+format+"\n", args...)
}

type syncCollect struct {
	git   scan.GitCollect
	files scan.FileCollect
}

func collectSync(cfg config.Config, db *store.DB, when syncWhen, dryRun bool) (syncCollect, error) {
	var gitCollected scan.GitCollect
	var err error
	if when.until.IsZero() {
		gitCollected, err = scan.CollectGit(db, rootPaths(cfg), cfg.Git.AuthorEmails, when.now, dryRun)
	} else {
		gitCollected, err = scan.CollectGitUntil(db, rootPaths(cfg), cfg.Git.AuthorEmails, when.now, when.until, dryRun)
	}
	if err != nil {
		return syncCollect{}, err
	}
	var filesCollected scan.FileCollect
	if when.until.IsZero() {
		filesCollected, err = scan.CollectFiles(db, rootPaths(cfg), cfg.ExcludeGlobs, when.now, dryRun, committedRepos(gitCollected))
	} else {
		filesCollected, err = scan.CollectFilesUntil(db, rootPaths(cfg), cfg.ExcludeGlobs, when.now, when.until, dryRun, committedRepos(gitCollected))
	}
	if err != nil {
		return syncCollect{}, err
	}
	return syncCollect{git: gitCollected, files: filesCollected}, nil
}

func committedRepos(collected scan.GitCollect) []string {
	seen := make(map[string]struct{})
	repos := make([]string, 0, len(collected.Commits))
	for _, commit := range collected.Commits {
		if _, ok := seen[commit.RepoPath]; ok {
			continue
		}
		seen[commit.RepoPath] = struct{}{}
		repos = append(repos, commit.RepoPath)
	}
	return repos
}

func rootPaths(cfg config.Config) []string {
	roots := make([]string, len(cfg.Roots))
	for i, root := range cfg.Roots {
		roots[i] = root.Path
	}
	return roots
}

func printCollect(w io.Writer, collected syncCollect, dryRun bool, wrote int) error {
	if dryRun {
		return printDryRun(w, collected)
	}
	if _, err := fmt.Fprintf(w, "sync: recorded %d commits, %d file touches\n",
		collected.git.Inserted,
		collected.files.Inserted+collected.files.Updated,
	); err != nil {
		return err
	}
	if wrote == 0 {
		return nil
	}
	noun := "bullets"
	if wrote == 1 {
		noun = "bullet"
	}
	_, err := fmt.Fprintf(w, "sync: wrote %d %s\n", wrote, noun)
	return err
}

func printDryRun(w io.Writer, collected syncCollect) error {
	if len(collected.git.Commits) == 0 {
		if _, err := fmt.Fprintln(w, "sync: no commits"); err != nil {
			return err
		}
	} else {
		for _, commit := range collected.git.Commits {
			_, err := fmt.Fprintf(w, "%s %s %s %s %s\n",
				commit.RepoPath,
				commit.Hash,
				commit.Time.Format(time.RFC3339),
				commit.Branch,
				commit.Subject,
			)
			if err != nil {
				return err
			}
		}
	}
	for _, touch := range collected.files.Touches {
		_, err := fmt.Fprintf(w, "%s file-touch %d %s\n", touch.Root, touch.Count, touch.Sample)
		if err != nil {
			return err
		}
	}
	return nil
}
