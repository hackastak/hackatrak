// Package launchd installs the per-user LaunchAgent that runs hackatrak sync
// at 12:00 and 23:00 local time.
package launchd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Runner runs a launchctl invocation and returns its combined output.
// A nil Runner uses exec.Command.
type Runner func(ctx context.Context, name string, args ...string) (string, error)

// LoadMethod is which launchctl subcommand accepted the agent.
type LoadMethod string

const (
	// MethodBootstrap is launchctl bootstrap (macOS 10.11+).
	MethodBootstrap LoadMethod = "bootstrap"
	// MethodLoad is the legacy launchctl load -w fallback.
	MethodLoad LoadMethod = "load"
)

// Exec runs name with args and includes combined output in any error.
func Exec(ctx context.Context, name string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return text, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return text, nil
}

// CurrentProgram is this process's executable, with symlinks resolved.
func CurrentProgram() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("launchd: executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

// IsEphemeralProgram reports a go run / go test cache binary.
// Those paths disappear, so a LaunchAgent must not point at them.
func IsEphemeralProgram(path string) bool {
	return strings.Contains(path, string(filepath.Separator)+"go-build"+string(filepath.Separator))
}

// Install writes the plist, creates the log directory, and loads the agent.
// An already-loaded job is booted out first so a reinstall picks up the new
// plist. bootstrap is preferred; load -w is the legacy fallback.
func Install(ctx context.Context, agent Agent, runner Runner) (LoadMethod, error) {
	ctx = background(ctx)
	runner = orExec(runner)
	if err := checkProgram(agent.Program); err != nil {
		return "", err
	}
	body, err := Render(agent)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(agent.Stdout), 0o755); err != nil {
		return "", fmt.Errorf("install: log dir: %w", err)
	}
	if err := writeAtomic(agent.PlistPath, body, 0o644); err != nil {
		return "", fmt.Errorf("install: write plist: %w", err)
	}
	method, err := enable(ctx, runner, agent)
	if err != nil {
		return "", fmt.Errorf("install: wrote %s but launchctl failed: %w", agent.PlistPath, err)
	}
	return method, nil
}

// Uninstall unloads the agent and removes its plist.
// A job that is already unloaded, and a plist that is already gone, succeed.
func Uninstall(ctx context.Context, agent Agent, runner Runner) error {
	ctx = background(ctx)
	runner = orExec(runner)
	if err := unload(ctx, runner, agent); err != nil {
		return err
	}
	if agent.PlistPath == "" {
		return nil
	}
	if err := os.Remove(agent.PlistPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("uninstall: remove plist: %w", err)
	}
	return nil
}

// Start runs a loaded agent now. It tries `launchctl start <label>` first,
// then kickstart on the gui service target.
func Start(ctx context.Context, agent Agent, runner Runner) error {
	ctx = background(ctx)
	runner = orExec(runner)
	label := agent.Label
	if label == "" {
		label = Label
	}
	if _, err := runner(ctx, "launchctl", "start", label); err == nil {
		return nil
	} else if _, kickErr := runner(ctx, "launchctl", "kickstart", agent.Service()); kickErr != nil {
		return fmt.Errorf("launchd: start: %w; kickstart: %v", err, kickErr)
	}
	return nil
}

// Print returns `launchctl print` for the loaded service.
func Print(ctx context.Context, agent Agent, runner Runner) (string, error) {
	ctx = background(ctx)
	runner = orExec(runner)
	out, err := runner(ctx, "launchctl", "print", agent.Service())
	if err != nil {
		return out, fmt.Errorf("launchd: print: %w", err)
	}
	return out, nil
}

func enable(ctx context.Context, runner Runner, agent Agent) (LoadMethod, error) {
	_, _ = runner(ctx, "launchctl", "enable", agent.Service())
	_, bootoutErr := runner(ctx, "launchctl", "bootout", agent.Service())
	_, err := runner(ctx, "launchctl", "bootstrap", Domain(), agent.PlistPath)
	if err == nil {
		return MethodBootstrap, nil
	}
	_, _ = runner(ctx, "launchctl", "unload", "-w", agent.PlistPath)
	if _, loadErr := runner(ctx, "launchctl", "load", "-w", agent.PlistPath); loadErr != nil {
		return "", fmt.Errorf("bootstrap: %w; load: %v; bootout: %v", err, loadErr, bootoutErr)
	}
	return MethodLoad, nil
}

func unload(ctx context.Context, runner Runner, agent Agent) error {
	_, err := runner(ctx, "launchctl", "bootout", agent.Service())
	if err == nil || serviceMissing(err.Error()) {
		return nil
	}
	if agent.PlistPath == "" {
		return fmt.Errorf("uninstall: bootout: %w", err)
	}
	_, unloadErr := runner(ctx, "launchctl", "unload", "-w", agent.PlistPath)
	if unloadErr == nil || serviceMissing(unloadErr.Error()) {
		return nil
	}
	return fmt.Errorf("uninstall: bootout: %w; unload: %v", err, unloadErr)
}

func serviceMissing(msg string) bool {
	msg = strings.ToLower(msg)
	for _, needle := range []string{
		"no such process",
		"could not find service",
		"could not find specified service",
		"service not found",
		"not loaded",
		"no such file or directory",
		"boot-out failed: 3",
		"unload failed: 5",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func checkProgram(path string) error {
	if IsEphemeralProgram(path) {
		return fmt.Errorf("install: refusing ephemeral binary %s; run make install and install that binary", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("install: program: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("install: program %s is a directory", path)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf("install: program %s is not executable", path)
	}
	return nil
}

func writeAtomic(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".hackatrak-plist-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func orExec(runner Runner) Runner {
	if runner == nil {
		return Exec
	}
	return runner
}

func background(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
