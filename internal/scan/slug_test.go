package scan

import (
	"path/filepath"
	"testing"
)

func TestWikilinkSlugRepoDirName(t *testing.T) {
	got, err := WikilinkSlug(filepath.Join("/Users/hackastak/Developer", "staksmith"))
	if err != nil {
		t.Fatalf("WikilinkSlug() error = %v", err)
	}
	if got != "staksmith" {
		t.Fatalf("WikilinkSlug() = %q, want staksmith", got)
	}
}

func TestWikilinkSlugKeepsHyphensAndCase(t *testing.T) {
	got, err := WikilinkSlug("/Users/i568439/Developer/supply-chain-monitor/")
	if err != nil {
		t.Fatalf("WikilinkSlug() error = %v", err)
	}
	if got != "supply-chain-monitor" {
		t.Fatalf("WikilinkSlug() = %q, want supply-chain-monitor", got)
	}
}

func TestWikilinkSlugNestedRepoDir(t *testing.T) {
	got, err := WikilinkSlug(filepath.Join("/Users/hackastak/Developer/monorepo/apps", "web"))
	if err != nil {
		t.Fatalf("WikilinkSlug() error = %v", err)
	}
	if got != "web" {
		t.Fatalf("WikilinkSlug() = %q, want web", got)
	}
}

func TestWikilinkSlugKeepsSpaces(t *testing.T) {
	got, err := WikilinkSlug(filepath.Join("/tmp/Developer", "My Repo"))
	if err != nil {
		t.Fatalf("WikilinkSlug() error = %v", err)
	}
	if got != "My Repo" {
		t.Fatalf("WikilinkSlug() = %q, want %q", got, "My Repo")
	}
}

func TestWikilinkSlugEmptyAndDot(t *testing.T) {
	for _, path := range []string{"", "   ", ".", "..", string(filepath.Separator)} {
		if _, err := WikilinkSlug(path); err == nil {
			t.Fatalf("WikilinkSlug(%q) error = nil", path)
		}
	}
}
