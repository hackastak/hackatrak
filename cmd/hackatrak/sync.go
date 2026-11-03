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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print detected commits without recording them")
	return cmd
}

// runSync is the collect step. Later phases add file presence, derivation,
// and the vault write after the git scan. --dry-run stops at printing.
func runSync(cmd *cobra.Command, dryRun bool) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	collected, err := collectGit(state.cfg, state.db, time.Now(), dryRun)
	if err != nil {
		return err
	}
	return printGitCollect(cmd.OutOrStdout(), collected, dryRun)
}

func collectGit(cfg config.Config, db *store.DB, now time.Time, dryRun bool) (scan.GitCollect, error) {
	return scan.CollectGit(db, rootPaths(cfg), cfg.Git.AuthorEmails, now, dryRun)
}

func rootPaths(cfg config.Config) []string {
	roots := make([]string, len(cfg.Roots))
	for i, root := range cfg.Roots {
		roots[i] = root.Path
	}
	return roots
}

func printGitCollect(w io.Writer, collected scan.GitCollect, dryRun bool) error {
	if dryRun {
		if len(collected.Commits) == 0 {
			_, err := fmt.Fprintln(w, "sync: no commits")
			return err
		}
		for _, commit := range collected.Commits {
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
		return nil
	}
	_, err := fmt.Fprintf(w, "sync: recorded %d commits\n", collected.Inserted)
	return err
}
