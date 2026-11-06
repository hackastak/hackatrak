package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// TestPhase2 is the Phase 2 acceptance gate (P2.T): a modified file in a
// repo with no commit today is presence, and a stray .DS_Store or build
// artifact does not trigger it.
func TestPhase2(t *testing.T) {
	root := t.TempDir()
	worked := filepath.Join(root, "worked")
	noise := filepath.Join(root, "noise")
	if err := os.MkdirAll(worked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(noise, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, worked, "init", "-b", "main")
	git(t, noise, "init", "-b", "main")

	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.Local)
	writeFileAt(t, filepath.Join(worked, "main.go"), now.Add(-time.Hour))
	writeFileAt(t, filepath.Join(worked, ".DS_Store"), now)
	writeFileAt(t, filepath.Join(worked, "dist", "app.js"), now)
	writeFileAt(t, filepath.Join(worked, "build", "hackatrak"), now)
	writeFileAt(t, filepath.Join(noise, ".DS_Store"), now)
	writeFileAt(t, filepath.Join(noise, "target", "debug", "app"), now)
	writeFileAt(t, filepath.Join(root, ".DS_Store"), now)

	db := openScanDB(t)
	commits, err := CollectGit(db, []string{root}, []string{"me@example.com"}, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits.Commits) != 0 || commits.Inserted != 0 {
		t.Fatalf("git collect = %+v, want no commits", commits)
	}

	got, err := CollectFiles(db, []string{root}, nil, now, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	workedPath := resolved(t, worked)
	if got.Inserted != 1 || got.Updated != 0 || len(got.Touches) != 1 {
		t.Fatalf("collect = %+v, want one file presence for the uncommitted edit", got)
	}
	touch := got.Touches[0]
	if touch.Root != workedPath || touch.Count != 1 || touch.Sample != "main.go" {
		t.Fatalf("touch = %+v, want only main.go; .DS_Store and build output must not count", touch)
	}

	events, err := db.EventsByRepo(workedPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != store.KindFileTouch || events[0].FileCount != 1 || events[0].FileSample != "main.go" {
		t.Fatalf("events = %+v, want one file-touch for main.go", events)
	}
	for _, repo := range []string{resolved(t, noise), resolved(t, root)} {
		stored, err := db.EventsByRepo(repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(stored) != 0 {
			t.Fatalf("noise at %s stored %+v, want no presence", repo, stored)
		}
	}
}
