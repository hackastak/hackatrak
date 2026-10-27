package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version and commit are set with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build version and git commit",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "hackatrak %s (%s)\n", version, commit)
		},
	}
}
