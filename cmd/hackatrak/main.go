// Command hackatrak is a local-first, automatic work log.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hackatrak",
		Short: "Local-first, automatic work log",
	}
	cmd.AddCommand(
		newSyncCmd(),
		newTodayCmd(),
		newSummaryCmd(),
		newLogCmd(),
		newInstallCmd(),
		newVersionCmd(),
	)
	return cmd
}
