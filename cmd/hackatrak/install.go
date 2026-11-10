package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/launchd"
)

// launchRunner is replaced by tests. Nil uses launchd.Exec.
var launchRunner launchd.Runner

func newInstallCmd() *cobra.Command {
	var binary string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the launchd agent (12:00 and 23:00 local)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstall(cmd, binary)
		},
	}
	cmd.Flags().StringVar(&binary, "binary", "", "absolute path of the hackatrak binary to schedule (default: this executable)")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Unload and remove the launchd agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUninstall(cmd)
		},
	}
}

func runInstall(cmd *cobra.Command, binary string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("install: launchd agents are macOS-only")
	}
	program, err := resolveProgram(binary)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	agent := launchd.DefaultAgent(home, program)
	method, err := launchd.Install(cmd.Context(), agent, launchRunner)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "installed %s\nloaded %s via launchctl %s\nschedule 12:00 and 23:00 local\nlogs %s\nlogs %s\n",
		agent.PlistPath, agent.Service(), method, agent.Stdout, agent.Stderr)
	return err
}

func runUninstall(cmd *cobra.Command) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("uninstall: launchd agents are macOS-only")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	agent := launchd.Agent{
		Label:     launchd.Label,
		PlistPath: launchd.PlistPath(home),
	}
	if err := launchd.Uninstall(cmd.Context(), agent, launchRunner); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "unloaded %s\nremoved %s\n", agent.Service(), agent.PlistPath)
	return err
}

func resolveProgram(binary string) (string, error) {
	program := binary
	if program == "" {
		found, err := launchd.CurrentProgram()
		if err != nil {
			return "", fmt.Errorf("install: %w", err)
		}
		program = found
	}
	abs, err := filepath.Abs(program)
	if err != nil {
		return "", fmt.Errorf("install: binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if launchd.IsEphemeralProgram(abs) {
		return "", fmt.Errorf("install: refusing ephemeral binary %s; run make install and install that binary", abs)
	}
	return abs, nil
}
