package launchd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// Label is the LaunchAgent label written by hackatrak install.
	Label = "com.hackastak.hackatrak"

	// PlistName is the filename under ~/Library/LaunchAgents.
	PlistName = Label + ".plist"

	templateName = "com.hackastak.hackatrak.plist.tmpl"
)

// CalendarHours are the local wall-clock hours in the authored plist.
// Minute is always 0. The template has no TimeZone key: launchd uses the
// system timezone, follows DST, and runs a missed interval once on wake.
var CalendarHours = []int{12, 23}

// Agent is one rendered LaunchAgent. Paths are absolute.
// Env is extra environment for the job. PATH is filled when missing.
// A nil Env does not override HOME; launchd keeps the logged-in user's home.
type Agent struct {
	Label     string
	Program   string
	Stdout    string
	Stderr    string
	PlistPath string
	Env       map[string]string
}

type envVar struct {
	Key   string
	Value string
}

type plistView struct {
	Label   string
	Program string
	Stdout  string
	Stderr  string
	Env     []envVar
}

// DefaultAgent is the per-user agent install writes.
func DefaultAgent(home, program string) Agent {
	stdout, stderr := LogPaths(home)
	return Agent{
		Label:     Label,
		Program:   program,
		Stdout:    stdout,
		Stderr:    stderr,
		PlistPath: PlistPath(home),
	}
}

// PlistPath is ~/Library/LaunchAgents/com.hackastak.hackatrak.plist.
func PlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", PlistName)
}

// LogPaths are the StandardOutPath and StandardErrorPath for sync.
func LogPaths(home string) (string, string) {
	dir := filepath.Join(home, "Library", "Logs", "hackatrak")
	return filepath.Join(dir, "sync.out.log"), filepath.Join(dir, "sync.err.log")
}

// Domain is the gui launchd domain for this user.
func Domain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

// Service is the launchctl target gui/<uid>/<label>.
func (a Agent) Service() string {
	label := a.Label
	if label == "" {
		label = Label
	}
	return Domain() + "/" + label
}

// Render fills the LaunchAgent template. ProgramArguments are the absolute
// program and "sync". Calendar intervals stay the authored 12:00 and 23:00.
func Render(agent Agent) ([]byte, error) {
	view, err := agent.view()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("launchd: render plist: %w", err)
	}
	return buf.Bytes(), nil
}

func (a Agent) view() (plistView, error) {
	label := a.Label
	if label == "" {
		label = Label
	}
	if err := checkLabel(label); err != nil {
		return plistView{}, err
	}
	program := strings.TrimSpace(a.Program)
	if program == "" || !filepath.IsAbs(program) {
		return plistView{}, fmt.Errorf("launchd: program must be an absolute path")
	}
	stdout := strings.TrimSpace(a.Stdout)
	stderr := strings.TrimSpace(a.Stderr)
	if stdout == "" || !filepath.IsAbs(stdout) || stderr == "" || !filepath.IsAbs(stderr) {
		return plistView{}, fmt.Errorf("launchd: log paths must be absolute")
	}
	return plistView{
		Label:   label,
		Program: program,
		Stdout:  stdout,
		Stderr:  stderr,
		Env:     environment(program, a.Env),
	}, nil
}

func environment(program string, extra map[string]string) []envVar {
	env := make(map[string]string, len(extra)+1)
	for key, value := range extra {
		env[key] = value
	}
	if _, ok := env["PATH"]; !ok {
		env["PATH"] = defaultPATH(program)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]envVar, 0, len(keys))
	for _, key := range keys {
		out = append(out, envVar{Key: key, Value: env[key]})
	}
	return out
}

func defaultPATH(program string) string {
	base := os.Getenv("PATH")
	if base == "" {
		base = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	}
	dir := filepath.Dir(program)
	if dir == "" || dir == "." || pathHasDir(base, dir) {
		return base
	}
	return dir + string(os.PathListSeparator) + base
}

func pathHasDir(path, dir string) bool {
	for _, part := range filepath.SplitList(path) {
		if part == dir {
			return true
		}
	}
	return false
}

func checkLabel(label string) error {
	if label == "" || strings.Contains(label, "/") || strings.ContainsAny(label, " \t") {
		return fmt.Errorf("launchd: label %q is not a single LaunchAgent label", label)
	}
	return nil
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}
