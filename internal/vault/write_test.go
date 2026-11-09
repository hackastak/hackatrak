package vault

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAtomicRereadsAndRenames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-W45.md")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(dir, "hardlink.md")); err != nil {
		t.Fatal(err)
	}

	var seen string
	err := WriteAtomic(path, func(note string) (string, error) {
		seen = note
		return note + "- bullet\n", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != "fresh\n" {
		t.Fatalf("splice saw %q, want the latest file, not a stale buffer", seen)
	}
	if got := readFile(t, path); got != "fresh\n- bullet\n" {
		t.Fatalf("note = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "hardlink.md")); got != "fresh\n" {
		t.Fatalf("hard link = %q, want the pre-rename inode (write was not atomic)", got)
	}
	assertNoTemp(t, dir)
}

func TestWriteAtomicRetriesChangedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "week.md")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	calls := 0
	err := WriteAtomic(path, func(note string) (string, error) {
		calls++
		if calls == 1 {
			if note != "original\n" {
				t.Fatalf("first read = %q", note)
			}
			if err := os.WriteFile(path, []byte("synced\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return "from-original\n", nil
		}
		if note != "synced\n" {
			t.Fatalf("re-read = %q, want the file as it was immediately before write", note)
		}
		return "from-synced\n", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want a re-read retry", calls)
	}
	if got := readFile(t, path); got != "from-synced\n" {
		t.Fatalf("note = %q", got)
	}
	assertNoTemp(t, dir)
}

func TestWriteAtomicGivesUpWithoutStaleSplice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "week.md")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := WriteAtomic(path, func(note string) (string, error) {
		calls++
		if err := os.WriteFile(path, []byte("ext-"+note), 0o644); err != nil {
			t.Fatal(err)
		}
		return "APPLIED\n", nil
	})
	if !errors.Is(err, ErrNoteChanged) {
		t.Fatalf("err = %v, want ErrNoteChanged", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want %d", calls, 3)
	}
	got := readFile(t, path)
	if got == "APPLIED\n" || strings.Contains(got, "APPLIED") {
		t.Fatalf("stale splice was written: %q", got)
	}
	if !strings.HasPrefix(got, "ext-") {
		t.Fatalf("file = %q, want the other writer's content", got)
	}
	assertNoTemp(t, dir)
}

func TestWriteAtomicLeavesFileOnSpliceError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "week.md")
	const original = "# Hackastak\n\n#### Daily Journal\nMonday\n- hand\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	err = WriteAtomic(path, func(string) (string, error) {
		return "should-not-land\n", ErrBucketNotFound
	})
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("err = %v", err)
	}
	if got := readFile(t, path); got != original {
		t.Fatalf("note = %q", got)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("failed splice rewrote the note")
	}
	assertNoTemp(t, dir)

	if err := WriteAtomic(path, nil); err == nil {
		t.Fatal("nil splice succeeded")
	}
	if err := WriteAtomic("  ", func(string) (string, error) { return "", nil }); err == nil {
		t.Fatal("empty path succeeded")
	}
	missing := filepath.Join(dir, "missing.md")
	err = WriteAtomic(missing, func(string) (string, error) { return "x", nil })
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file err = %v", err)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing note was created")
	}
}

func TestWriteAtomicSkipsUnchangedAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "week.md")
	if err := os.WriteFile(path, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := WriteAtomic(path, func(note string) (string, error) { return note, nil }); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged note was rewritten")
	}

	if err := WriteAtomic(path, func(note string) (string, error) { return note + "more\n", nil }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}
	assertNoTemp(t, dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	assertNoTemp(t, cwd)
}

func TestWriteAtomicSpliceOnFixture(t *testing.T) {
	path := copyFixture(t, "2026-W45.md")
	original := readFile(t, path)
	saturday := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	bullet := Bullet{Slug: "supply-chain-monitor", Text: "- [[supply-chain-monitor]]: must not land"}
	var skipped bytes.Buffer
	err := WriteAtomic(path, func(note string) (string, error) {
		return Splice(note, "SAP Business Network & ANT", saturday, bullet, SpliceOptions{
			Missing: SkipMissing,
			Log:     log.New(&skipped, "", 0),
		})
	})
	if !errors.Is(err, ErrWeekdayNotFound) {
		t.Fatalf("err = %v", err)
	}
	if readFile(t, path) != original {
		t.Fatal("skip wrote the note")
	}
	if !strings.Contains(skipped.String(), "skipping splice") || !strings.Contains(skipped.String(), "Saturday") {
		t.Fatalf("log = %q", skipped.String())
	}
	assertNoTemp(t, filepath.Dir(path))

	var created bytes.Buffer
	err = WriteAtomic(path, func(note string) (string, error) {
		return Splice(note, "SAP Business Network & ANT", saturday, bullet, SpliceOptions{
			Missing: CreateMissing,
			Log:     log.New(&created, "", 0),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.Contains(created.String(), "created weekday heading \"Saturday\"") {
		t.Fatalf("log = %q", created.String())
	}
	section, err := LocateSection(got, "SAP Business Network & ANT", saturday)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spanText(got, section), "<!-- ht:supply-chain-monitor -->") {
		t.Fatalf("saturday =\n%s", spanText(got, section))
	}
	if strings.Count(got, "- OOO - advisory board meeting") != 1 {
		t.Fatal("hand-written Friday bullet was not preserved")
	}
	assertNoTemp(t, filepath.Dir(path))

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := WriteAtomic(path, func(note string) (string, error) {
		return Splice(note, "SAP Business Network & ANT", saturday, bullet, SpliceOptions{Missing: CreateMissing})
	}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != got {
		t.Fatal("idempotent re-run changed the note")
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("idempotent re-run rewrote the file")
	}
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, readFixture(t, name), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".hackatrak-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}
