package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureWeekNoteCreatesFromTemplate(t *testing.T) {
	vaultPath := t.TempDir()
	installTemplate(t, vaultPath, readFixture(t, "Weekly_note.md"))

	day := time.Date(2026, 1, 7, 16, 45, 0, 0, mustLocation(t, "America/Chicago"))
	path, err := EnsureWeekNote(vaultPath, day)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(vaultPath, "_Weekly", "2026", "2026-W02.md")
	if path != wantPath {
		t.Fatalf("path = %s, want %s", path, wantPath)
	}

	got := readFile(t, path)
	if strings.Contains(got, "<%") || strings.Contains(got, "undefined") {
		t.Fatalf("created note still has templater output:\n%s", got)
	}
	const wantDateLine = "_Notes for the week of 2026-01-05._  "
	if !strings.Contains(got, wantDateLine) {
		t.Fatalf("date line missing %q\n%s", wantDateLine, got)
	}
	template := string(readFixture(t, "Weekly_note.md"))
	rest := strings.TrimPrefix(template, "_Notes for the week of <% tp.date.weekday(\"YYYY-MM-DD\", 1) %>._  ")
	if rest == template {
		t.Fatal("fixture date line did not match the real template")
	}
	if !strings.HasSuffix(got, rest) {
		t.Fatal("created note changed template content other than the date line")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 644", info.Mode().Perm())
	}
}

func TestEnsureWeekNoteYearBoundary(t *testing.T) {
	tests := []struct {
		name     string
		day      time.Time
		relPath  string
		dateLine string
	}{
		{
			name:     "late december lands in next week-year",
			day:      time.Date(2025, 12, 31, 21, 0, 0, 0, time.UTC),
			relPath:  "_Weekly/2026/2026-W01.md",
			dateLine: "_Notes for the week of 2025-12-29._",
		},
		{
			name:     "week 53 file uses the previous week-year",
			day:      time.Date(2021, 1, 3, 11, 0, 0, 0, time.UTC),
			relPath:  "_Weekly/2020/2020-W53.md",
			dateLine: "_Notes for the week of 2020-12-28._",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultPath := t.TempDir()
			installTemplate(t, vaultPath, []byte("_Notes for the week of <% tp.date.weekday(\"YYYY-MM-DD\", 1) %>._\n# Hackastak\n"))
			path, err := EnsureWeekNote(vaultPath, tt.day)
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(vaultPath, filepath.FromSlash(tt.relPath))
			if path != want {
				t.Fatalf("path = %s, want %s", path, want)
			}
			got := readFile(t, path)
			if !strings.Contains(got, tt.dateLine) {
				t.Fatalf("note = %q, want date line %q", got, tt.dateLine)
			}
			if strings.Contains(got, "<%") || strings.Contains(got, "undefined") {
				t.Fatalf("templater leaked into %s", path)
			}
		})
	}
}

func TestEnsureWeekNoteSubstitutesUndefined(t *testing.T) {
	vaultPath := t.TempDir()
	installTemplate(t, vaultPath, []byte("_Notes for the week of undefined._\n# Hackastak\n"))

	path, err := EnsureWeekNote(vaultPath, time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "undefined") || strings.Contains(got, "<%") {
		t.Fatalf("undefined leaked:\n%s", got)
	}
	if !strings.Contains(got, "_Notes for the week of 2026-01-05._\n# Hackastak\n") {
		t.Fatalf("note = %q", got)
	}
}

func TestEnsureWeekNoteLeavesExistingFile(t *testing.T) {
	vaultPath := t.TempDir()
	installTemplate(t, vaultPath, readFixture(t, "Weekly_note.md"))
	day := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(vaultPath, "_Weekly", "2026", "2026-W45.md")
	existing := "- hand written, no marker\n<% not ours %>\nundefined stays\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	gotPath, err := EnsureWeekNote(vaultPath, day)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %s, want %s", gotPath, path)
	}
	if got := readFile(t, path); got != existing {
		t.Fatalf("existing note changed:\n%s", got)
	}

	again, err := EnsureWeekNote(vaultPath, day)
	if err != nil {
		t.Fatal(err)
	}
	if again != path || readFile(t, path) != existing {
		t.Fatal("second ensure rewrote the existing note")
	}
}

func TestEnsureWeekNoteMissingTemplate(t *testing.T) {
	vaultPath := t.TempDir()
	_, err := EnsureWeekNote(vaultPath, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected missing template error")
	}
	matches, globErr := filepath.Glob(filepath.Join(vaultPath, "_Weekly", "*", "*.md"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("created notes despite missing template: %v", matches)
	}
}

func TestEnsureWeekNoteRequiresDateLine(t *testing.T) {
	vaultPath := t.TempDir()
	installTemplate(t, vaultPath, []byte("# Hackastak\nMonday\n"))
	_, err := EnsureWeekNote(vaultPath, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNoDateLine) {
		t.Fatalf("err = %v, want ErrNoDateLine", err)
	}
}

func installTemplate(t *testing.T, vaultPath string, body []byte) {
	t.Helper()
	dir := filepath.Join(vaultPath, "_templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Weekly_note.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
