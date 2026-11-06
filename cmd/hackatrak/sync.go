package main

import (
	"fmt"
	"io"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/scan"
	"github.com/hackastak/hackatrak/internal/store"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Scan configured roots and record work",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSync(cmd, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print detected commits and file presence without recording them")
	return cmd
}

// runSync collects git commits and file-mtime presence. A repo with a commit
// today is not also recorded as a file-touch. Later phases derive presence
// and write the vault. --dry-run stops at printing.
func runSync(cmd *cobra.Command, dryRun bool) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	collected, err := collectSync(state.cfg, state.db, time.Now(), dryRun)
	if err != nil {
		return err
	}
	return printCollect(cmd.OutOrStdout(), collected, dryRun)
}

type syncCollect struct {
	git   scan.GitCollect
	files scan.FileCollect
}

func collectSync(cfg config.Config, db *store.DB, now time.Time, dryRun bool) (syncCollect, error) {
	gitCollected, err := scan.CollectGit(db, rootPaths(cfg), cfg.Git.AuthorEmails, now, dryRun)
	if err != nil {
		return syncCollect{}, err
	}
	filesCollected, err := scan.CollectFiles(db, rootPaths(cfg), cfg.ExcludeGlobs, now, dryRun, committedRepos(gitCollected))
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

func printCollect(w io.Writer, collected syncCollect, dryRun bool) error {
	if dryRun {
		if err := printDryRun(w, collected); err != nil {
			return err
		}
		return nil
	}
	_, err := fmt.Fprintf(w, "sync: recorded %d commits, %d file touches\n",
		collected.git.Inserted,
		collected.files.Inserted+collected.files.Updated,
	)
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
