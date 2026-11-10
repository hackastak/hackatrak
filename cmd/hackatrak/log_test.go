package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogSplicesManualBullet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	if err := os.MkdirAll(filepath.Join(root, "staksmith"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before := readNote(t, notePath)
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "log", "staksmith: paired the manual escape hatch")
	if err != nil {
		t.Fatalf("log: %v\n%s", err, out)
	}
	if strings.Contains(out, "not implemented") {
		t.Fatalf("log is still a stub:\n%s", out)
	}
	note := readNote(t, notePath)
	want := "- [[staksmith]]: paired the manual escape hatch <!-- ht:staksmith -->"
	today := time.Now().Weekday().String()
	section := weekdaySection(t, note, "Hackastak", today)
	if !strings.Contains(section, want) {
		t.Fatalf("Hackastak %s = %q, want %s", today, section, want)
	}
	if strings.Count(note, "<!-- ht:staksmith -->") != 1 {
		t.Fatalf("marker count = %d\n%s", strings.Count(note, "<!-- ht:staksmith -->"), note)
	}
	if !strings.Contains(note, "set up production repo infra") {
		t.Fatal("log removed an unmarked line")
	}
	if strings.Contains(bucketSection(note, "SAP Business Network & ANT"), "ht:staksmith") {
		t.Fatal("manual bullet landed in the other bucket")
	}

	again, err := execute(t, "log", "staksmith (NEXTGEN-9): replaced the manual line")
	if err != nil {
		t.Fatalf("second log: %v\n%s", err, again)
	}
	replaced := readNote(t, notePath)
	if strings.Count(replaced, "<!-- ht:staksmith -->") != 1 {
		t.Fatalf("re-log duplicated the marker:\n%s", replaced)
	}
	section = weekdaySection(t, replaced, "Hackastak", today)
	if strings.Contains(section, "paired the manual escape hatch") {
		t.Fatalf("old manual line still present:\n%s", section)
	}
	if !strings.Contains(section, "- [[staksmith]] (NEXTGEN-9): replaced the manual line <!-- ht:staksmith -->") {
		t.Fatalf("replaced line missing:\n%s", section)
	}
	if replaced == before {
		t.Fatal("log did not change the note")
	}
}

func TestLogRejectsUnknownProjectAndSpooler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before := readNote(t, notePath)
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "log", "missing: did X")
	if err == nil {
		t.Fatalf("unknown project succeeded: %s", out)
	}
	if readNote(t, notePath) != before {
		t.Fatal("unknown project changed the note")
	}

	body := "vault_path = " + quote(vaultPath) + "\nrole = \"spooler\"\naccount = \"i568439\"\n\n[[roots]]\npath = " + quote(root) + "\nbucket = \"SAP Business Network & ANT\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n"
	writeConfigBody(t, home, body)
	if err := os.MkdirAll(filepath.Join(root, "staksmith"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err = execute(t, "log", "staksmith: did X")
	if err == nil || !strings.Contains(err.Error(), "spooler") {
		t.Fatalf("spooler log = %v\n%s", err, out)
	}
	if readNote(t, notePath) != before {
		t.Fatal("spooler log changed the note")
	}
}

func TestLogRequiresTheEntryShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeValidConfig(t, home)

	out, err := execute(t, "log")
	if err == nil {
		t.Fatalf("log with no args succeeded: %s", out)
	}
	out, err = execute(t, "log", "no colon here")
	if err == nil {
		t.Fatalf("log without a colon succeeded: %s", out)
	}
}
