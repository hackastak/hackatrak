package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/spool"
)

// TestPhase7HandPlacedSpoolRoundTrip is the phase 7 gate.
// A spool file placed by hand (not written by this process) lands in the
// SAP bucket with a marker. A later spooler run, even with its own commit,
// leaves that note byte-for-byte untouched.
func TestPhase7HandPlacedSpoolRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	writerRoot := filepath.Join(home, "Developer")
	if err := os.MkdirAll(writerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before := readNote(t, notePath)

	now := time.Now()
	year, week := now.ISOWeek()
	const slug = "batch-reconciler"
	const ticket = "NEXTGEN-9"
	const visible = "- [[batch-reconciler]] (NEXTGEN-9): hand-placed spool round trip"
	spoolName, err := spool.Name("i568439", year, week)
	if err != nil {
		t.Fatal(err)
	}
	spoolDir := filepath.Join(vaultPath, config.DefaultSpoolDir)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`{
  "version": 1,
  "account": "i568439",
  "year": %d,
  "week": %d,
  "days": [
    {
      "date": %q,
      "bucket": "SAP Business Network & ANT",
      "slug": %q,
      "ticket": %q,
      "text": %q
    }
  ]
}
`, year, week, now.Format("2006-01-02"), slug, ticket, visible)
	spoolPath := filepath.Join(spoolDir, spoolName)
	if err := os.WriteFile(spoolPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfigBody(t, home, "vault_path = "+quote(vaultPath)+"\nrole = \"writer\"\naccount = \"hackastak\"\n\n[[roots]]\npath = "+quote(writerRoot)+"\nbucket = \"Hackastak\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("writer sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: wrote 1 bullet\n") {
		t.Fatalf("writer output = %q, want one SAP bullet from the hand-placed spool", out)
	}
	note := readNote(t, notePath)
	weekday := now.Weekday().String()
	sap := weekdaySection(t, note, sapBucket, weekday)
	want := visible + " <!-- ht:" + slug + " -->"
	if !strings.Contains(sap, want) {
		t.Fatalf("SAP %s = %q, want %q", weekday, sap, want)
	}
	if strings.Count(note, "<!-- ht:"+slug+" -->") != 1 {
		t.Fatalf("marker count = %d, want 1:\n%s", strings.Count(note, "<!-- ht:"+slug+" -->"), note)
	}
	if strings.Contains(bucketSection(note, "Hackastak"), "ht:"+slug) {
		t.Fatalf("spool bullet landed outside SAP:\n%s", note)
	}
	if !strings.Contains(note, "reviewed the unentitled read path") || !strings.Contains(note, "set up production repo infra") {
		t.Fatalf("hand-written lines were not preserved:\n%s", note)
	}
	if !strings.Contains(sap, "<!-- ht:supply-chain-monitor -->") && strings.Contains(before, "<!-- ht:supply-chain-monitor -->") && weekday == "Monday" {
		t.Fatalf("existing SAP marker was dropped:\n%s", sap)
	}

	again, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("writer re-run: %v\n%s", err, again)
	}
	if strings.Contains(again, "sync: wrote") {
		t.Fatalf("writer re-run wrote bullets: %q", again)
	}
	if readNote(t, notePath) != note {
		t.Fatal("writer re-run changed the note")
	}
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("writer consumed the spool file: %v", err)
	}

	workRoot := filepath.Join(home, "Work")
	workRepo := filepath.Join(workRoot, "supply-chain-monitor")
	if err := os.MkdirAll(workRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, workRepo, "init", "-b", "main")
	gitCommitAt(t, workRepo, commitOnToday(-time.Minute), "me@example.com", "Me", "spooler must not splice")
	writeConfigBody(t, home, "vault_path = "+quote(vaultPath)+"\nrole = \"spooler\"\naccount = \"i568439\"\n\n[[roots]]\npath = "+quote(workRoot)+"\nbucket = \"SAP Business Network & ANT\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n")

	spooled, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("spooler sync: %v\n%s", err, spooled)
	}
	if strings.Contains(spooled, "sync: wrote") {
		t.Fatalf("spooler reported a note write: %q", spooled)
	}
	if !strings.Contains(spooled, "sync: spooled ") {
		t.Fatalf("spooler output = %q, want a spool write and no note write", spooled)
	}
	if readNote(t, notePath) != note {
		t.Fatal("spooler run changed the weekly note")
	}
}
