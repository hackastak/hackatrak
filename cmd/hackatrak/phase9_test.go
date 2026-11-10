package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPhase9ReviewAndManualLog is the phase 9 gate.
// today and summary, including --json, match the bullet sync wrote into the
// note. A manual log lands on the same marker path and a second run does not
// duplicate it.
func TestPhase9ReviewAndManualLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	gitCmd(t, repo, "init", "-b", "NEXTGEN-42")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "wire the review")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	note := readNote(t, notePath)
	today := time.Now().Weekday().String()
	section := weekdaySection(t, note, "Hackastak", today)
	wantBullet := "- [[demo]] (NEXTGEN-42): wire the review <!-- ht:demo -->"
	if !strings.Contains(section, wantBullet) {
		t.Fatalf("note %s = %q, want %s", today, section, wantBullet)
	}

	review, err := execute(t, "today")
	if err != nil {
		t.Fatalf("today: %v\n%s", err, review)
	}
	assertReviewMatchesNote(t, review, "demo", "NEXTGEN-42", "wire the review")
	summary, err := execute(t, "summary")
	if err != nil {
		t.Fatalf("summary: %v\n%s", err, summary)
	}
	assertReviewMatchesNote(t, summary, "demo", "NEXTGEN-42", "wire the review")

	encoded, err := execute(t, "today", "--json")
	if err != nil {
		t.Fatalf("today --json: %v\n%s", err, encoded)
	}
	doc := decodeReview(t, encoded)
	if len(doc.Projects) != 1 || doc.Projects[0].Project != "demo" || doc.Projects[0].Ticket != "NEXTGEN-42" || doc.Projects[0].Work != "wire the review" {
		t.Fatalf("json review = %+v", doc.Projects)
	}

	logged, err := execute(t, "log", "notes: wrote the manual line")
	if err != nil {
		t.Fatalf("log: %v\n%s", err, logged)
	}
	note = readNote(t, notePath)
	manual := "- [[notes]]: wrote the manual line <!-- ht:notes -->"
	section = weekdaySection(t, note, "Hackastak", today)
	if !strings.Contains(section, manual) || !strings.Contains(section, wantBullet) {
		t.Fatalf("manual log missing beside the sync bullet:\n%s", section)
	}
	if strings.Count(note, "<!-- ht:notes -->") != 1 {
		t.Fatalf("manual marker count = %d\n%s", strings.Count(note, "<!-- ht:notes -->"), note)
	}
	if !strings.Contains(note, "set up production repo infra") {
		t.Fatal("log removed an unmarked line")
	}

	again, err := execute(t, "log", "notes: wrote the manual line")
	if err != nil {
		t.Fatalf("second log: %v\n%s", err, again)
	}
	reread := readNote(t, notePath)
	if reread != note {
		t.Fatal("re-log changed the note")
	}
	if strings.Count(reread, "<!-- ht:notes -->") != 1 {
		t.Fatalf("re-log duplicated the manual marker:\n%s", reread)
	}
}

func assertReviewMatchesNote(t *testing.T, out, project, ticket, work string) {
	t.Helper()
	for _, want := range []string{project, ticket, work} {
		if !strings.Contains(out, want) {
			t.Fatalf("review missing %q:\n%s", want, out)
		}
	}
}
