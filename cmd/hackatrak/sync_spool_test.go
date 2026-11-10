package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/spool"
	"github.com/hackastak/hackatrak/internal/vault"
)

func TestSyncSpoolerWritesSpoolAndLeavesNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "supply-chain-monitor")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "NEXTGEN-9")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "spool the work day")
	writeConfigBody(t, home, "vault_path = "+quote(vaultPath)+"\nrole = \"spooler\"\naccount = \"i568439\"\nticket_regex = 'NEXTGEN-\\d+'\n\n[[roots]]\npath = "+quote(root)+"\nbucket = \"SAP Business Network & ANT\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if strings.Contains(out, "sync: wrote") {
		t.Fatalf("spooler reported a note write: %q", out)
	}
	after, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("spooler changed the weekly note")
	}

	now := time.Now()
	path, err := spool.Path(filepath.Join(vaultPath, config.DefaultSpoolDir), "i568439", now)
	if err != nil {
		t.Fatal(err)
	}
	file, err := spool.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if file.Account != "i568439" || len(file.Days) != 1 {
		t.Fatalf("spool = %#v", file)
	}
	day := file.Days[0]
	if day.Date != now.Format("2006-01-02") || day.Bucket != sapBucket || day.Slug != "supply-chain-monitor" {
		t.Fatalf("day = %#v", day)
	}
	if day.Ticket != "NEXTGEN-9" || strings.Contains(day.Text, "<!-- ht:") {
		t.Fatalf("day text = %#v", day)
	}
	if !strings.Contains(day.Text, "- [[supply-chain-monitor]] (NEXTGEN-9): spool the work day") {
		t.Fatalf("text = %q", day.Text)
	}

	again, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("second sync: %v\n%s", err, again)
	}
	reread, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reread, before) {
		t.Fatal("second spooler run changed the weekly note")
	}
}

func TestSyncSpoolerIdleDoesNotCreateSpoolOrNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vaultPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfigBody(t, home, "vault_path = "+quote(vaultPath)+"\nrole = \"spooler\"\naccount = \"i568439\"\n\n[[roots]]\npath = "+quote(root)+"\nbucket = \"SAP Business Network & ANT\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("idle spooler created the note: %v", err)
	}
	spoolPath, err := spool.Path(filepath.Join(vaultPath, config.DefaultSpoolDir), "i568439", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(spoolPath); !os.IsNotExist(err) {
		t.Fatalf("idle spooler created %s: %v", spoolPath, err)
	}
}

func TestSyncWriterMergesSpoolIntoSAP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "personal line")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	now := time.Now()
	year, week := now.ISOWeek()
	spoolDir := filepath.Join(vaultPath, config.DefaultSpoolDir)
	spoolPath, err := spool.Path(spoolDir, "i568439", now)
	if err != nil {
		t.Fatal(err)
	}
	otherWeek, err := spool.Name("other", year, week%53+1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spoolDir, otherWeek), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = spool.Write(spoolPath, spool.File{
		Version: spool.Version,
		Account: "i568439",
		Year:    year,
		Week:    week,
		Days: []spool.Day{
			{
				Date:   now.Format("2006-01-02"),
				Bucket: "SAP",
				Slug:   "supply-chain-monitor",
				Ticket: "NEXTGEN-9",
				Text:   "- [[supply-chain-monitor]] (NEXTGEN-9): arrived from the spool",
			},
			{
				Date:   now.Format("2006-01-02"),
				Bucket: "Hackastak",
				Slug:   "not-sap",
				Text:   "- [[not-sap]]: must stay out of the note",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: wrote 2 bullets") {
		t.Fatalf("output = %q, want own bullet plus SAP spool bullet", out)
	}
	note := readNote(t, notePath)
	weekday := now.Weekday().String()
	sap := weekdaySection(t, note, sapBucket, weekday)
	if !strings.Contains(sap, "- [[supply-chain-monitor]] (NEXTGEN-9): arrived from the spool <!-- ht:supply-chain-monitor -->") {
		t.Fatalf("SAP %s = %q", weekday, sap)
	}
	if strings.Contains(note, "ht:not-sap") || strings.Contains(note, "must stay out") {
		t.Fatalf("non-SAP spool day was spliced:\n%s", note)
	}
	own := weekdaySection(t, note, "Hackastak", weekday)
	if !strings.Contains(own, "- [[demo]]: personal line <!-- ht:demo -->") {
		t.Fatalf("Hackastak %s = %q", weekday, own)
	}
	if strings.Contains(sap, "ht:demo") {
		t.Fatalf("own bullet landed in SAP:\n%s", sap)
	}

	again, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("second sync: %v\n%s", err, again)
	}
	if strings.Contains(again, "sync: wrote") {
		t.Fatalf("re-run wrote bullets: %q", again)
	}
	reread := readNote(t, notePath)
	if strings.Count(reread, "<!-- ht:supply-chain-monitor -->") != 1 {
		t.Fatalf("re-run duplicated the SAP marker:\n%s", reread)
	}
	if reread != note {
		t.Fatal("re-run changed the note")
	}
	if !strings.Contains(reread, "set up production repo infra") && !bytes.Contains(before, []byte("set up production repo infra")) {
		t.Fatal("fixture lost an unmarked line")
	}
}
