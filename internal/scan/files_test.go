package scan

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFilesSinceKeepsFilesAtOrAfterMidnight(t *testing.T) {
	root := t.TempDir()
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)

	today := filepath.Join(root, "alpha", "main.go")
	exact := filepath.Join(root, "notes.txt")
	yesterday := filepath.Join(root, "alpha", "old.go")
	writeFileAt(t, today, since.Add(2*time.Hour))
	writeFileAt(t, exact, since)
	writeFileAt(t, yesterday, since.Add(-time.Second))

	got, err := FilesSince([]string{root}, since)
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}

	want := []string{
		resolved(t, today),
		resolved(t, exact),
	}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("FilesSince() mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}
	for _, file := range got {
		if file.Root != resolved(t, root) {
			t.Fatalf("Root = %q, want %q", file.Root, resolved(t, root))
		}
		if file.MTime.Before(since) {
			t.Fatalf("MTime %s is before since %s", file.MTime, since)
		}
	}
}

func TestFilesSinceExcludesNoise(t *testing.T) {
	root := t.TempDir()
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)
	keep := []string{
		filepath.Join(root, "src", "main.go"),
		filepath.Join(root, "build.go"),
		filepath.Join(root, "notes.md"),
	}
	noise := []string{
		filepath.Join(root, ".git", "objects", "ab"),
		filepath.Join(root, "dist", "app.js"),
		filepath.Join(root, "node_modules", "pkg", "index.js"),
		filepath.Join(root, "vendor", "lib", "lib.go"),
		filepath.Join(root, "build", "hackatrak"),
		filepath.Join(root, "target", "debug", "app"),
		filepath.Join(root, "coverage", "lcov.info"),
		filepath.Join(root, "__pycache__", "mod.pyc"),
		filepath.Join(root, ".next", "server.js"),
		filepath.Join(root, ".turbo", "cache.json"),
		filepath.Join(root, "src", "main.go.swp"),
		filepath.Join(root, "src", "notes.txt~"),
		filepath.Join(root, ".DS_Store"),
		filepath.Join(root, "src", ".DS_Store"),
		filepath.Join(root, "foo.o"),
		filepath.Join(root, "tmp", "cache.tmp"),
		filepath.Join(root, "secret", "token.txt"),
		filepath.Join(root, "scratch", "notes.md"),
	}
	for _, path := range append(append([]string{}, keep...), noise...) {
		writeFileAt(t, path, since.Add(time.Minute))
	}

	got, err := FilesSince([]string{root}, since, "*.tmp", "secret/*", "scratch")
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}
	want := make([]string, len(keep))
	for i, path := range keep {
		want[i] = resolved(t, path)
	}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("FilesSince() mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}
}

func TestFilesSinceHonorsGitignore(t *testing.T) {
	root := t.TempDir()
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)
	keep := []string{
		filepath.Join(root, "src", "main.go"),
		filepath.Join(root, "keep.log"),
		filepath.Join(root, "sub", "top.txt"),
		filepath.Join(root, "docs", "nested", "a.md"),
		filepath.Join(root, "abc", "keep.txt"),
		filepath.Join(root, "local.tmp"),
	}
	noise := []string{
		filepath.Join(root, "debug.log"),
		filepath.Join(root, "src", "debug.log"),
		filepath.Join(root, "top.txt"),
		filepath.Join(root, "secret", "hidden.go"),
		filepath.Join(root, "docs", "a.md"),
		filepath.Join(root, "abc", "other.txt"),
		filepath.Join(root, "sub", "local.tmp"),
		filepath.Join(root, "sub", "nested", "local.tmp"),
	}
	writeFileAt(t, filepath.Join(root, ".gitignore"), since.Add(time.Minute))
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(strings.Join([]string{
		"# comment",
		"*.log",
		"!keep.log",
		"secret/",
		"/top.txt",
		"docs/*.md",
		"abc/**",
		"!abc/keep.txt",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".gitignore"), []byte("local.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range append(append([]string{}, keep...), noise...) {
		writeFileAt(t, path, since.Add(time.Minute))
	}

	got, err := FilesSince([]string{root}, since)
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}
	want := make([]string, 0, len(keep)+2)
	for _, path := range keep {
		want = append(want, resolved(t, path))
	}
	want = append(want,
		resolved(t, filepath.Join(root, ".gitignore")),
		resolved(t, filepath.Join(root, "sub", ".gitignore")),
	)
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("FilesSince() mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}
}

func TestFilesSinceUsesRepoGitignoreAboveScanRoot(t *testing.T) {
	repo := t.TempDir()
	mustRepo(t, repo)
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "sub")
	keep := filepath.Join(sub, "main.go")
	noise := filepath.Join(sub, "debug.log")
	writeFileAt(t, keep, since.Add(time.Minute))
	writeFileAt(t, noise, since.Add(time.Minute))

	got, err := FilesSince([]string{sub}, since)
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}
	want := []string{resolved(t, keep)}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("repo gitignore mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}

	lose := t.TempDir()
	if err := os.WriteFile(filepath.Join(lose, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loseSub := filepath.Join(lose, "sub")
	loseLog := filepath.Join(loseSub, "debug.log")
	writeFileAt(t, loseLog, since.Add(time.Minute))
	got, err = FilesSince([]string{loseSub}, since)
	if err != nil {
		t.Fatalf("FilesSince(non-repo) error = %v", err)
	}
	want = []string{resolved(t, loseLog)}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("non-repo parent gitignore mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}
}

func TestFilesSinceEmptyRoot(t *testing.T) {
	got, err := FilesSince([]string{t.TempDir()}, time.Now())
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FilesSince() = %#v, want none", got)
	}
}

func TestFilesSinceRejectsBadArgs(t *testing.T) {
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)

	_, err := FilesSince(nil, since)
	if err == nil {
		t.Fatal("empty roots error = nil")
	}
	_, err = FilesSince([]string{"  "}, since)
	if err == nil {
		t.Fatal("blank root error = nil")
	}
	_, err = FilesSince([]string{t.TempDir()}, time.Time{})
	if err == nil || err.Error() != "files since: time is required" {
		t.Fatalf("zero since error = %v", err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	_, err = FilesSince([]string{missing}, since)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root error = %v, want os.ErrNotExist", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = FilesSince([]string{file}, since)
	if !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("file root error = %v, want ErrNotDirectory", err)
	}

	_, err = FilesSince([]string{t.TempDir()}, since, "[")
	if err == nil || !errors.Is(err, path.ErrBadPattern) {
		t.Fatalf("bad glob error = %v, want path.ErrBadPattern", err)
	}
	_, err = FilesSince([]string{t.TempDir()}, since, "  ")
	if err == nil || !strings.Contains(err.Error(), "exclude glob is empty") {
		t.Fatalf("blank glob error = %v", err)
	}
}

func TestFilesSinceResolvesRelativeAndSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)
	file := filepath.Join(root, "main.go")
	writeFileAt(t, file, since.Add(time.Minute))

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FilesSince([]string{rel}, since)
	if err != nil {
		t.Fatalf("FilesSince(rel) error = %v", err)
	}
	want := []string{resolved(t, file)}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("relative root mismatch:\n%s", diff)
	}
	if got[0].Root != resolved(t, root) {
		t.Fatalf("Root = %q, want resolved root", got[0].Root)
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	got, err = FilesSince([]string{link}, since)
	if err != nil {
		t.Fatalf("FilesSince(link) error = %v", err)
	}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("symlinked root mismatch:\n%s", diff)
	}
}

func TestFilesSinceSkipsSymlinksAndDedupesRoots(t *testing.T) {
	root := t.TempDir()
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.Local)
	repo := filepath.Join(root, "alpha")
	inside := filepath.Join(repo, "main.go")
	writeFileAt(t, inside, since.Add(time.Minute))

	outside := t.TempDir()
	hidden := filepath.Join(outside, "secret.go")
	writeFileAt(t, hidden, since.Add(time.Minute))
	if err := os.Symlink(hidden, filepath.Join(repo, "secret.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "elsewhere")); err != nil {
		t.Fatal(err)
	}

	got, err := FilesSince([]string{root, repo}, since)
	if err != nil {
		t.Fatalf("FilesSince() error = %v", err)
	}
	want := []string{resolved(t, inside)}
	if diff := missingOrExtra(touchedPaths(got), want); diff != "" {
		t.Fatalf("FilesSince() mismatch:\n%s\ngot %#v", diff, touchedPaths(got))
	}
	if got[0].Root != resolved(t, root) {
		t.Fatalf("Root = %q, want first root %q", got[0].Root, resolved(t, root))
	}
}

func touchedPaths(files []TouchedFile) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}

func writeFileAt(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}
