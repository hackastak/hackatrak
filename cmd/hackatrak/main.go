// Command hackatrak is a local-first, automatic work log.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "hackatrak",
		Short:              "Local-first, automatic work log",
		SilenceUsage:       true,
		PersistentPreRunE:  prepare,
		PersistentPostRunE: finish,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	cmd.AddCommand(
		newSyncCmd(),
		newTodayCmd(),
		newSummaryCmd(),
		newLogCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newVersionCmd(),
	)
	return cmd
}
