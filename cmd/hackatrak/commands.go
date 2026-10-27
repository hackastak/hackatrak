package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command    { return newStubCmd("sync") }
func newTodayCmd() *cobra.Command   { return newStubCmd("today") }
func newSummaryCmd() *cobra.Command { return newStubCmd("summary") }
func newLogCmd() *cobra.Command     { return newStubCmd("log") }
func newInstallCmd() *cobra.Command { return newStubCmd("install") }

func newStubCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: name + " (not implemented)",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "hackatrak %s: not implemented\n", name)
		},
	}
}
