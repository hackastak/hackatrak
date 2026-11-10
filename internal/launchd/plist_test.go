package launchd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTemplateIsLocalCalendarSchedule(t *testing.T) {
	raw, err := templateFS.ReadFile(templateName)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"ProgramArguments",
		"<string>sync</string>",
		"StartCalendarInterval",
		"<integer>12</integer>",
		"<integer>23</integer>",
		"<integer>0</integer>",
		"StandardOutPath",
		"StandardErrorPath",
		"once on wake",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("template missing %q", want)
		}
	}
	if strings.Contains(text, "<key>TimeZone</key>") {
		t.Fatal("TimeZone would pin the schedule off local wall-clock")
	}
	if strings.Contains(text, "StartInterval") {
		t.Fatal("StartInterval is not the calendar schedule")
	}
	if strings.Contains(text, "<key>RunAtLoad</key>") {
		t.Fatal("RunAtLoad would splice a note just because the agent was loaded")
	}
	if !bytes.Contains(raw, []byte("<integer>12</integer>")) || !bytes.Contains(raw, []byte("<integer>23</integer>")) {
		t.Fatal("authored hours drifted from 12:00 and 23:00")
	}
	if len(CalendarHours) != 2 || CalendarHours[0] != 12 || CalendarHours[1] != 23 {
		t.Fatalf("CalendarHours = %v, want [12 23]", CalendarHours)
	}
}

func TestRenderUsesAbsoluteProgramAndEscapesXML(t *testing.T) {
	agent := Agent{
		Program: "/opt/hack & co/hackatrak",
		Stdout:  "/var/log/hackatrak/sync.out.log",
		Stderr:  "/var/log/hackatrak/sync.err.log",
		Env:     map[string]string{"HOME": "/tmp/home&"},
	}
	body, err := Render(agent)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"<string>/opt/hack &amp; co/hackatrak</string>",
		"<string>sync</string>",
		"<key>HOME</key>",
		"<string>/tmp/home&amp;</string>",
		"<key>PATH</key>",
		"<integer>12</integer>",
		"<integer>23</integer>",
		"<string>" + Label + "</string>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered plist missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "<key>TimeZone</key>") {
		t.Fatal("TimeZone would pin the schedule off local wall-clock")
	}
	if strings.Count(text, "<string>sync</string>") != 1 {
		t.Fatalf("sync argument count drifted:\n%s", text)
	}
	lintPlist(t, body)
}

func TestRenderRejectsRelativeProgram(t *testing.T) {
	_, err := Render(Agent{Program: "hackatrak", Stdout: "/tmp/out", Stderr: "/tmp/err"})
	if err == nil {
		t.Fatal("relative program was accepted")
	}
}

func TestDefaultPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "ada")
	agent := DefaultAgent(home, filepath.Join(home, ".local", "bin", "hackatrak"))
	if agent.PlistPath != filepath.Join(home, "Library", "LaunchAgents", PlistName) {
		t.Fatalf("plist = %s", agent.PlistPath)
	}
	if !strings.HasSuffix(agent.Stdout, filepath.Join("Library", "Logs", "hackatrak", "sync.out.log")) {
		t.Fatalf("stdout = %s", agent.Stdout)
	}
	if !strings.HasSuffix(agent.Stderr, filepath.Join("Library", "Logs", "hackatrak", "sync.err.log")) {
		t.Fatalf("stderr = %s", agent.Stderr)
	}
	if agent.Service() != Domain()+"/"+Label {
		t.Fatalf("service = %s", agent.Service())
	}
}

func lintPlist(t *testing.T, body []byte) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil not available")
	}
	path := filepath.Join(t.TempDir(), "agent.plist")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(plutil, "-lint", path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil -lint: %v\n%s", err, out)
	}
}
