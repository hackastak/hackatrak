package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGitProjectNestedFile(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "staksmith")
	mustRepo(t, repo)
	file := filepath.Join(repo, "internal", "scan", "git.go")

	got, err := ResolveGitProject(file)
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, repo), Name: "staksmith"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectRepoRoot(t *testing.T) {
	repo := t.TempDir()
	mustRepo(t, repo)

	got, err := ResolveGitProject(repo)
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	wantRoot := resolved(t, repo)
	want := GitProject{Root: wantRoot, Name: filepath.Base(wantRoot)}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectNestedRepoWins(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "monorepo")
	inner := filepath.Join(outer, "apps", "web")
	mustRepo(t, outer)
	mustRepo(t, inner)
	file := filepath.Join(inner, "src", "main.go")

	got, err := ResolveGitProject(file)
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, inner), Name: "web"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectGitFile(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "worktree")
	mustRepoFile(t, repo)

	got, err := ResolveGitProject(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, repo), Name: "worktree"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectRelativePath(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "hackatrak")
	mustRepo(t, repo)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, filepath.Join(repo, "cmd", "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := ResolveGitProject(rel)
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, repo), Name: "hackatrak"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectInsideGitDir(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "tool")
	mustRepo(t, repo)

	got, err := ResolveGitProject(filepath.Join(repo, ".git", "objects"))
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, repo), Name: "tool"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectSymlinkedRepo(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "realname")
	mustRepo(t, repo)
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveGitProject(filepath.Join(link, "main.go"))
	if err != nil {
		t.Fatalf("ResolveGitProject() error = %v", err)
	}
	want := GitProject{Root: resolved(t, repo), Name: "realname"}
	if got != want {
		t.Fatalf("ResolveGitProject() = %+v, want %+v", got, want)
	}
}

func TestResolveGitProjectEmptyPath(t *testing.T) {
	_, err := ResolveGitProject("   ")
	if err == nil {
		t.Fatal("ResolveGitProject(empty) error = nil")
	}
}

func TestResolveProjectFallbackFirstSegment(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "notes", "drafts")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(notes, "a.md")

	got, err := ResolveProject(file, []string{root})
	if err != nil {
		t.Fatalf("ResolveProject() error = %v", err)
	}
	want := Project{Root: resolved(t, filepath.Join(root, "notes")), Name: "notes"}
	if got != want {
		t.Fatalf("ResolveProject() = %+v, want %+v", got, want)
	}
}

func TestResolveProjectGitWinsOverFirstSegment(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "monorepo", "apps", "web")
	mustRepo(t, repo)

	got, err := ResolveProject(filepath.Join(repo, "src", "main.go"), []string{root})
	if err != nil {
		t.Fatalf("ResolveProject() error = %v", err)
	}
	want := Project{Root: resolved(t, repo), Name: "web"}
	if got != want {
		t.Fatalf("ResolveProject() = %+v, want %+v", got, want)
	}
}

func TestResolveProjectLongestRoot(t *testing.T) {
	base := t.TempDir()
	broad := filepath.Join(base, "Developer")
	special := filepath.Join(broad, "vendor-src")
	file := filepath.Join(special, "dep", "lib", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveProject(file, []string{broad, special})
	if err != nil {
		t.Fatalf("ResolveProject() error = %v", err)
	}
	want := Project{Root: resolved(t, filepath.Join(special, "dep")), Name: "dep"}
	if got != want {
		t.Fatalf("ResolveProject() = %+v, want %+v", got, want)
	}
}

func TestResolveProjectEachAccountRoot(t *testing.T) {
	home := t.TempDir()
	hack := filepath.Join(home, "hackastak", "Developer")
	sap := filepath.Join(home, "i568439", "Developer")
	file := filepath.Join(sap, "jarvis", "notes", "a.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveProject(file, []string{hack, sap})
	if err != nil {
		t.Fatalf("ResolveProject() error = %v", err)
	}
	want := Project{Root: resolved(t, filepath.Join(sap, "jarvis")), Name: "jarvis"}
	if got != want {
		t.Fatalf("ResolveProject() = %+v, want %+v", got, want)
	}
}

func TestResolveProjectRootItself(t *testing.T) {
	root := t.TempDir()

	_, err := ResolveProject(root, []string{root})
	if !errors.Is(err, ErrNoProject) {
		t.Fatalf("ResolveProject() error = %v, want ErrNoProject", err)
	}
}

func TestResolveProjectOutsideRoots(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(t.TempDir(), "loose", "file.md")

	_, err := ResolveProject(other, []string{root})
	if !errors.Is(err, ErrNoProject) {
		t.Fatalf("ResolveProject() error = %v, want ErrNoProject", err)
	}
}

func TestResolveProjectDoesNotMatchPathPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Developer")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "DeveloperExtra", "proj", "a.md")

	_, err := ResolveProject(file, []string{root})
	if !errors.Is(err, ErrNoProject) {
		t.Fatalf("ResolveProject() error = %v, want ErrNoProject", err)
	}
}

func TestResolveGitProjectNoAncestor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notes", "loose")

	_, err := ResolveGitProject(dir)
	if !errors.Is(err, ErrNoGitProject) {
		t.Fatalf("ResolveGitProject() error = %v, want ErrNoGitProject", err)
	}
}
