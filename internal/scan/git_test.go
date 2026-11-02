package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRepos(t *testing.T) {
	root := t.TempDir()

	mustRepo(t, filepath.Join(root, "alpha"))
	mustRepo(t, filepath.Join(root, "beta"))
	mustRepo(t, filepath.Join(root, "beta", "nested"))
	mustRepo(t, filepath.Join(root, "vendor"))
	mustRepo(t, filepath.Join(root, "node_modules"))
	mustRepo(t, filepath.Join(root, "loose", "vendor", "dep"))
	mustRepo(t, filepath.Join(root, "loose", "node_modules", "pkg"))
	mustRepo(t, filepath.Join(root, "loose", "apps", "web"))
	mustRepoFile(t, filepath.Join(root, "worktree"))
	mustRepo(t, filepath.Join(root, ".hidden", "tool"))
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "alpha"), filepath.Join(root, "alpha-link")); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverRepos(root)
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}

	want := []string{
		resolved(t, filepath.Join(root, ".hidden", "tool")),
		resolved(t, filepath.Join(root, "alpha")),
		resolved(t, filepath.Join(root, "beta")),
		resolved(t, filepath.Join(root, "loose", "apps", "web")),
		resolved(t, filepath.Join(root, "worktree")),
	}
	if diff := missingOrExtra(got, want); diff != "" {
		t.Fatalf("DiscoverRepos() mismatch:\n%s\ngot %#v", diff, got)
	}
}

func TestDiscoverReposResolvesRelativeAndSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	mustRepo(t, filepath.Join(root, "proj"))

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverRepos(rel)
	if err != nil {
		t.Fatalf("DiscoverRepos(rel) error = %v", err)
	}
	want := []string{resolved(t, filepath.Join(root, "proj"))}
	if diff := missingOrExtra(got, want); diff != "" {
		t.Fatalf("relative root mismatch:\n%s", diff)
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	got, err = DiscoverRepos(link)
	if err != nil {
		t.Fatalf("DiscoverRepos(link) error = %v", err)
	}
	if diff := missingOrExtra(got, want); diff != "" {
		t.Fatalf("symlinked root mismatch:\n%s", diff)
	}
}

func TestDiscoverReposRootIsRepo(t *testing.T) {
	root := t.TempDir()
	mustRepo(t, root)
	mustRepo(t, filepath.Join(root, "nested"))

	got, err := DiscoverRepos(root)
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}
	want := []string{resolved(t, root)}
	if diff := missingOrExtra(got, want); diff != "" {
		t.Fatalf("root repo mismatch:\n%s", diff)
	}
}

func TestDiscoverReposEmptyRoot(t *testing.T) {
	got, err := DiscoverRepos(t.TempDir())
	if err != nil {
		t.Fatalf("DiscoverRepos() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DiscoverRepos() = %#v, want none", got)
	}
}

func TestDiscoverReposRejectsMissingAndFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := DiscoverRepos(missing)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root error = %v, want os.ErrNotExist", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = DiscoverRepos(file)
	if !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("file root error = %v, want ErrNotDirectory", err)
	}
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRepoFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func missingOrExtra(got, want []string) string {
	have := map[string]int{}
	for _, p := range got {
		have[p]++
	}
	var msg string
	for _, p := range want {
		if have[p] == 0 {
			msg += "missing " + p + "\n"
		}
		delete(have, p)
	}
	for p, n := range have {
		msg += "extra " + p
		if n > 1 {
			msg += " (duplicate)"
		}
		msg += "\n"
	}
	return msg
}
