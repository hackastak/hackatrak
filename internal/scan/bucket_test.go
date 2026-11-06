package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hackastak/hackatrak/internal/config"
)

func TestResolveBucketEachAccountRoot(t *testing.T) {
	base := t.TempDir()
	hack := filepath.Join(base, "hackastak", "Developer")
	sap := filepath.Join(base, "i568439", "Developer")
	if err := os.MkdirAll(filepath.Join(hack, "staksmith"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sap, "jarvis"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := []config.Root{
		{Path: hack, Bucket: "Hackastak"},
		{Path: sap, Bucket: "SAP Business Network & ANT"},
	}

	got, err := ResolveBucket(filepath.Join(sap, "jarvis", "main.go"), roots)
	if err != nil {
		t.Fatalf("ResolveBucket() error = %v", err)
	}
	if got != "SAP Business Network & ANT" {
		t.Fatalf("ResolveBucket() = %q, want SAP bucket", got)
	}

	got, err = ResolveBucket(filepath.Join(hack, "staksmith"), roots)
	if err != nil {
		t.Fatalf("ResolveBucket() error = %v", err)
	}
	if got != "Hackastak" {
		t.Fatalf("ResolveBucket() = %q, want Hackastak", got)
	}
}

func TestResolveBucketLongestPrefix(t *testing.T) {
	base := t.TempDir()
	broad := filepath.Join(base, "Developer")
	special := filepath.Join(broad, "work")
	file := filepath.Join(special, "proj", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveBucket(file, []config.Root{
		{Path: broad, Bucket: "Hackastak"},
		{Path: special, Bucket: "SAP Business Network & ANT"},
	})
	if err != nil {
		t.Fatalf("ResolveBucket() error = %v", err)
	}
	if got != "SAP Business Network & ANT" {
		t.Fatalf("ResolveBucket() = %q, want the longer root's bucket", got)
	}
}

func TestResolveBucketRootItself(t *testing.T) {
	root := t.TempDir()

	got, err := ResolveBucket(root, []config.Root{{Path: root, Bucket: "Hackastak"}})
	if err != nil {
		t.Fatalf("ResolveBucket() error = %v", err)
	}
	if got != "Hackastak" {
		t.Fatalf("ResolveBucket() = %q, want Hackastak", got)
	}
}

func TestResolveBucketDoesNotMatchStringPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Developer")
	file := filepath.Join(base, "DeveloperExtra", "proj", "a.md")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveBucket(file, []config.Root{{Path: root, Bucket: "Hackastak"}})
	if !errors.Is(err, ErrNoBucket) {
		t.Fatalf("ResolveBucket() error = %v, want ErrNoBucket", err)
	}
}

func TestResolveBucketOutsideRoots(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(t.TempDir(), "loose", "file.md")

	_, err := ResolveBucket(other, []config.Root{{Path: root, Bucket: "Hackastak"}})
	if !errors.Is(err, ErrNoBucket) {
		t.Fatalf("ResolveBucket() error = %v, want ErrNoBucket", err)
	}
}

func TestResolveBucketEmptyPath(t *testing.T) {
	_, err := ResolveBucket("   ", []config.Root{{Path: t.TempDir(), Bucket: "Hackastak"}})
	if err == nil {
		t.Fatal("ResolveBucket(empty) error = nil")
	}
	if errors.Is(err, ErrNoBucket) {
		t.Fatalf("ResolveBucket(empty) error = %v, want a path error", err)
	}
}

func TestResolveBucketEmptyBucket(t *testing.T) {
	root := t.TempDir()

	_, err := ResolveBucket(filepath.Join(root, "proj"), []config.Root{{Path: root, Bucket: "  "}})
	if err == nil {
		t.Fatal("ResolveBucket() error = nil, want empty bucket")
	}
	if errors.Is(err, ErrNoBucket) {
		t.Fatalf("ResolveBucket() error = %v, want empty-bucket error", err)
	}
}
