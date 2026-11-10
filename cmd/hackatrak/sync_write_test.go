package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
	"github.com/hackastak/hackatrak/internal/vault"
)

func TestSyncWriterSplicesOwnBucketOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	before, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "NEXTGEN-42")
	gitCommitAt(t, repo, commitOnToday(-2*time.Minute), "me@example.com", "Me", "wire the writer")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: wrote 1 bullet") {
		t.Fatalf("output = %q, want one written bullet", out)
	}

	note := readNote(t, notePath)
	want := "- [[demo]] (NEXTGEN-42): wire the writer <!-- ht:demo -->"
	today := time.Now().Weekday().String()
	section := weekdaySection(t, note, "Hackastak", today)
	if !strings.Contains(section, want) {
		t.Fatalf("Hackastak %s = %q, want %s", today, section, want)
	}
	if strings.Count(note, "<!-- ht:demo -->") != 1 {
		t.Fatalf("demo marker count = %d, want 1\n%s", strings.Count(note, "<!-- ht:demo -->"), note)
	}
	if today == "Sunday" && !strings.Contains(section, "condense commit subjects into one line <!-- ht:hackatrak -->") {
		t.Fatalf("hand-written Sunday line was not preserved:\n%s", section)
	}
	if bucketSection(note, "SAP Business Network & ANT") != bucketSection(string(before), "SAP Business Network & ANT") {
		t.Fatalf("SAP bucket changed:\n%s", bucketSection(note, "SAP Business Network & ANT"))
	}

	again, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("second sync: %v\n%s", err, again)
	}
	if strings.Contains(again, "sync: wrote") {
		t.Fatalf("idle re-run wrote a bullet: %q", again)
	}
	reread := readNote(t, notePath)
	if strings.Count(reread, "<!-- ht:demo -->") != 1 {
		t.Fatalf("re-run duplicated the bullet:\n%s", reread)
	}
	if reread != note {
		t.Fatal("re-run changed the note")
	}

	db := openCLIStore(t, home)
	presence, err := db.DayPresence("demo", time.Now().Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	if !presence.Worked || presence.Source != store.PresenceCommit || presence.Ticket != "NEXTGEN-42" {
		t.Fatalf("presence = %+v", presence)
	}
}

func TestSyncWriterReplacesMarkedLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	notePath := placeWeekFixture(t, vaultPath, "2026-W45.md")
	stale := insertUnderWeekday(t, readNote(t, notePath), "Hackastak", time.Now().Weekday().String(),
		"- [[demo]] (NEXTGEN-1): stale summary <!-- ht:demo -->")
	if err := os.WriteFile(notePath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-b", "NEXTGEN-42")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "replace the marked line")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	note := readNote(t, notePath)
	if strings.Count(note, "<!-- ht:demo -->") != 1 {
		t.Fatalf("marker count = %d, want a replacement not a second line\n%s", strings.Count(note, "<!-- ht:demo -->"), note)
	}
	if strings.Contains(note, "stale summary") {
		t.Fatalf("stale marked line survived:\n%s", note)
	}
	today := time.Now().Weekday().String()
	section := weekdaySection(t, note, "Hackastak", today)
	if !strings.Contains(section, "- [[demo]] (NEXTGEN-42): replace the marked line <!-- ht:demo -->") {
		t.Fatalf("%s = %q", today, section)
	}
	if !strings.Contains(note, "set up production repo infra") {
		t.Fatal("unmarked line was removed")
	}
}

func TestSyncWriterCreatesMissingWeek(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "create the week file")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("note already exists: %v", err)
	}

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	note := readNote(t, notePath)
	if strings.Contains(note, "undefined") || strings.Contains(note, "<%") {
		t.Fatalf("created note still has templater output:\n%s", note)
	}
	if !strings.Contains(note, weekMonday(time.Now()).Format("2006-01-02")) {
		t.Fatalf("created note missing Monday date:\n%s", note)
	}
	today := time.Now().Weekday().String()
	section := weekdaySection(t, note, "Hackastak", today)
	if !strings.Contains(section, "- [[demo]]: create the week file <!-- ht:demo -->") {
		t.Fatalf("Hackastak %s = %q", today, section)
	}
	if strings.Contains(bucketSection(note, "SAP Business Network & ANT"), "ht:demo") {
		t.Fatalf("missing-week create wrote the own-bucket bullet into SAP:\n%s", note)
	}
}

func TestSyncWriterFileOnlyBullet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	gitCmd(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().Weekday().String()
	section := weekdaySection(t, readNote(t, notePath), "Hackastak", today)
	if !strings.Contains(section, "- [[demo]]: touched 1 file (main.go) <!-- ht:demo -->") {
		t.Fatalf("%s = %q, want file fallback bullet", today, section)
	}
}

func TestSyncWriterKeepsBucketsSeparate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	hackRoot := filepath.Join(home, "Developer")
	sapRoot := filepath.Join(home, "Work")
	hackRepo := filepath.Join(hackRoot, "demo")
	sapRepo := filepath.Join(sapRoot, "supply-chain-monitor")
	for _, dir := range []string{hackRepo, sapRepo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "init", "-b", "main")
	}
	gitCommitAt(t, hackRepo, commitOnToday(-2*time.Minute), "me@example.com", "Me", "personal work")
	gitCommitAt(t, sapRepo, commitOnToday(-time.Minute), "me@example.com", "Me", "work account scan")
	copyWeeklyTemplate(t, vaultPath)
	writeConfigBody(t, home, ""+
		"vault_path = "+quote(vaultPath)+"\n"+
		"role = \"writer\"\n"+
		"account = \"hackastak\"\n\n"+
		"[[roots]]\npath = "+quote(hackRoot)+"\nbucket = \"Hackastak\"\n\n"+
		"[[roots]]\npath = "+quote(sapRoot)+"\nbucket = \"SAP Business Network & ANT\"\n\n"+
		"[git]\nauthor_emails = [\"me@example.com\"]\n")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if !strings.Contains(out, "sync: wrote 2 bullets") {
		t.Fatalf("output = %q, want two bullets", out)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	note := readNote(t, notePath)
	today := time.Now().Weekday().String()
	hack := weekdaySection(t, note, "Hackastak", today)
	sap := weekdaySection(t, note, "SAP Business Network & ANT", today)
	if !strings.Contains(hack, "<!-- ht:demo -->") || strings.Contains(hack, "ht:supply-chain-monitor") {
		t.Fatalf("Hackastak Sunday = %q", hack)
	}
	if !strings.Contains(sap, "<!-- ht:supply-chain-monitor -->") || strings.Contains(sap, "ht:demo") {
		t.Fatalf("SAP Sunday = %q", sap)
	}
}

func TestSyncSpoolerDoesNotTouchNote(t *testing.T) {
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
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "spooler must not splice")
	writeConfigBody(t, home, "vault_path = "+quote(vaultPath)+"\nrole = \"spooler\"\naccount = \"i568439\"\n\n[[roots]]\npath = "+quote(root)+"\nbucket = \"Hackastak\"\n\n[git]\nauthor_emails = [\"me@example.com\"]\n")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if strings.Contains(out, "sync: wrote") {
		t.Fatalf("spooler wrote a bullet: %q", out)
	}
	after, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("spooler changed the weekly note")
	}
}

func TestSyncDryRunDoesNotWriteNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	gitCmd(t, repo, "init", "-b", "main")
	gitCommitAt(t, repo, commitOnToday(-time.Minute), "me@example.com", "Me", "dry run only")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync --dry-run: %v\n%s", err, out)
	}
	if strings.Contains(out, "sync: wrote") || strings.Contains(out, "recorded") {
		t.Fatalf("dry-run output = %q", out)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created %s: %v", notePath, err)
	}
}

func TestSyncWriterIdleDoesNotCreateNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vaultPath := filepath.Join(home, "vault")
	root := filepath.Join(home, "Developer")
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	copyWeeklyTemplate(t, vaultPath)
	gitCmd(t, repo, "init", "-b", "main")
	writeWriterConfig(t, home, vaultPath, root, "Hackastak")

	out, err := execute(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	if strings.Contains(out, "sync: wrote") {
		t.Fatalf("idle sync wrote a bullet: %q", out)
	}
	notePath, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("idle sync created %s: %v", notePath, err)
	}
}

func writeWriterConfig(t *testing.T, home, vaultPath, root, bucket string) {
	t.Helper()
	body := "vault_path = " + quote(vaultPath) + "\nrole = \"writer\"\naccount = \"hackastak\"\nticket_regex = 'NEXTGEN-\\d+'\n\n[[roots]]\npath = " + quote(root) + "\nbucket = " + quote(bucket) + "\n\n[git]\nauthor_emails = [\"me@example.com\"]\n"
	writeConfigBody(t, home, body)
}

func copyWeeklyTemplate(t *testing.T, vaultPath string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "vault", "testdata", "Weekly_note.md"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(vaultPath, "_templates", "Weekly_note.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func placeWeekFixture(t *testing.T, vaultPath, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "vault", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	path, err := vault.NotePath(vaultPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readNote(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func bucketSection(note, bucket string) string {
	heading := "# " + bucket
	idx := strings.Index(note, heading)
	if idx < 0 {
		return ""
	}
	rest := note[idx+len(heading):]
	if i := strings.Index(rest, "\n# "); i >= 0 {
		return rest[:i]
	}
	return rest
}

func weekdaySection(t *testing.T, note, bucket, weekday string) string {
	t.Helper()
	body := bucketSection(note, bucket)
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == weekday {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("weekday %s missing under %s", weekday, bucket)
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "Monday" || line == "Tuesday" || line == "Wednesday" || line == "Thursday" || line == "Friday" || line == "Saturday" || line == "Sunday" {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func insertUnderWeekday(t *testing.T, note, bucket, weekday, line string) string {
	t.Helper()
	body := bucketSection(note, bucket)
	idx := strings.Index(note, body)
	if idx < 0 || body == "" {
		t.Fatalf("bucket %s missing", bucket)
	}
	rel := strings.Index(body, "\n"+weekday+"\n")
	if rel < 0 {
		t.Fatalf("weekday %s missing under %s", weekday, bucket)
	}
	at := idx + rel + len("\n"+weekday+"\n")
	return note[:at] + line + "\n" + note[at:]
}

func weekMonday(day time.Time) time.Time {
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	return midnight.AddDate(0, 0, -(int(midnight.Weekday())+6)%7)
}
