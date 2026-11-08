package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hackastak/hackatrak/internal/config"
)

// TestPhase3 is the Phase 3 acceptance gate (P3.T): table-driven paths map
// to the expected project, bucket, and wikilink slug.
func TestPhase3(t *testing.T) {
	base := t.TempDir()
	hack := filepath.Join(base, "hackastak", "Developer")
	sap := filepath.Join(base, "i568439", "Developer")
	roots := []config.Root{
		{Path: hack, Bucket: "Hackastak"},
		{Path: sap, Bucket: "SAP Business Network & ANT"},
	}
	rootPaths := []string{hack, sap}

	nestedPersonal := filepath.Join(hack, "monorepo", "apps", "web")
	repoAtPersonalRoot := filepath.Join(hack, "staksmith")
	nestedWork := filepath.Join(sap, "platform", "services", "api")
	repoAtWorkRoot := filepath.Join(sap, "supply-chain-monitor")
	mustRepo(t, nestedPersonal)
	mustRepo(t, repoAtPersonalRoot)
	mustRepo(t, nestedWork)
	mustRepo(t, repoAtWorkRoot)

	personalNotes := filepath.Join(hack, "scratch", "ideas", "a.md")
	workNotes := filepath.Join(sap, "notes", "drafts", "a.md")
	for _, file := range []string{
		filepath.Join(nestedPersonal, "src", "main.go"),
		filepath.Join(repoAtPersonalRoot, "README.md"),
		personalNotes,
		filepath.Join(nestedWork, "cmd", "main.go"),
		filepath.Join(repoAtWorkRoot, "go.mod"),
		workNotes,
	} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		path    string
		root    string
		project string
		bucket  string
		slug    string
	}{
		{
			name:    "nested repo under personal root",
			path:    filepath.Join(nestedPersonal, "src", "main.go"),
			root:    nestedPersonal,
			project: "web",
			bucket:  "Hackastak",
			slug:    "web",
		},
		{
			name:    "repo directly under personal root",
			path:    filepath.Join(repoAtPersonalRoot, "README.md"),
			root:    repoAtPersonalRoot,
			project: "staksmith",
			bucket:  "Hackastak",
			slug:    "staksmith",
		},
		{
			name:    "non-git dir under personal root",
			path:    personalNotes,
			root:    filepath.Join(hack, "scratch"),
			project: "scratch",
			bucket:  "Hackastak",
			slug:    "scratch",
		},
		{
			name:    "nested repo under work root",
			path:    filepath.Join(nestedWork, "cmd", "main.go"),
			root:    nestedWork,
			project: "api",
			bucket:  "SAP Business Network & ANT",
			slug:    "api",
		},
		{
			name:    "repo directly under work root",
			path:    filepath.Join(repoAtWorkRoot, "go.mod"),
			root:    repoAtWorkRoot,
			project: "supply-chain-monitor",
			bucket:  "SAP Business Network & ANT",
			slug:    "supply-chain-monitor",
		},
		{
			name:    "non-git dir under work root",
			path:    workNotes,
			root:    filepath.Join(sap, "notes"),
			project: "notes",
			bucket:  "SAP Business Network & ANT",
			slug:    "notes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveProject(tt.path, rootPaths)
			if err != nil {
				t.Fatalf("ResolveProject() error = %v", err)
			}
			wantRoot := resolved(t, tt.root)
			if got.Root != wantRoot || got.Name != tt.project || got.Slug != tt.slug {
				t.Fatalf("project = %+v, want root %s name %s slug %s", got, wantRoot, tt.project, tt.slug)
			}
			bucket, err := ResolveBucket(tt.path, roots)
			if err != nil {
				t.Fatalf("ResolveBucket() error = %v", err)
			}
			if bucket != tt.bucket {
				t.Fatalf("bucket = %q, want %q", bucket, tt.bucket)
			}
			slug, err := WikilinkSlug(got.Root)
			if err != nil {
				t.Fatalf("WikilinkSlug() error = %v", err)
			}
			if slug != tt.slug {
				t.Fatalf("wikilink slug = %q, want %q", slug, tt.slug)
			}
		})
	}
}
