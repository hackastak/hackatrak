package summarize

import (
	"strings"
	"testing"

	"github.com/hackastak/hackatrak/internal/config"
)

// TestPhase5 is the Phase 5 acceptance gate (P5.T): the config-selected
// mechanical summarizer turns a commit day into one line plus a ticket,
// a no-commit day into file text, and a noisy pile of commits into a
// readable condensation. The line never carries the ticket parentheses.
func TestPhase5(t *testing.T) {
	s, err := FromConfig(config.Config{
		Summarizer:  config.SummarizerMechanical,
		TicketRegex: `NEXTGEN-\d+`,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("commits become one line plus a ticket", func(t *testing.T) {
		day := Day{
			Branch: "feature/NEXTGEN-54211-probe",
			Commits: []Commit{
				{Subject: "fix: traced the 504.", Branch: "feature/NEXTGEN-1"},
				{Subject: "feat(scan): ported the allow-rule"},
			},
			FileCount:  2,
			FileSample: "probe.go, allow.go",
		}
		got := summarizeTwice(t, s, day)
		if got.Line != "traced the 504; ported the allow-rule" {
			t.Fatalf("Line = %q", got.Line)
		}
		if got.Ticket != "NEXTGEN-54211" {
			t.Fatalf("Ticket = %q, want the branch id, not a commit id", got.Ticket)
		}
		assertNoTicketParens(t, got)
	})

	t.Run("no-commit day is file text", func(t *testing.T) {
		day := Day{
			Branch:     "main",
			FileCount:  3,
			FileSample: "a.go, b.go",
		}
		got := summarizeTwice(t, s, day)
		if got.Line != "touched 3 files (a.go, b.go, …)" {
			t.Fatalf("Line = %q", got.Line)
		}
		if got.Ticket != "" {
			t.Fatalf("Ticket = %q, want empty when nothing matches", got.Ticket)
		}
	})

	t.Run("many commits condense", func(t *testing.T) {
		day := Day{
			Branch: "bugfix/NEXTGEN-88",
			Commits: []Commit{
				{Subject: "Merge branch 'main'"},
				{Subject: "fix: " + letters(70, 'a')},
				{Subject: "chore: " + letters(70, 'a')},
				{Subject: letters(70, 'b') + "."},
				{Subject: letters(40, 'c')},
				{Subject: "docs: note the seam"},
				{Subject: "   "},
			},
		}
		got := summarizeTwice(t, s, day)
		want := letters(70, 'a') + "; " + letters(70, 'b') + " (+2 more)"
		if got.Line != want {
			t.Fatalf("Line = %q, want %q", got.Line, want)
		}
		if strings.Contains(got.Line, "Merge") || strings.Contains(got.Line, "fix:") {
			t.Fatalf("Line = %q, want merges and type prefixes dropped", got.Line)
		}
		if got.Ticket != "NEXTGEN-88" {
			t.Fatalf("Ticket = %q", got.Ticket)
		}
		assertNoTicketParens(t, got)
	})
}

func summarizeTwice(t *testing.T, s Summarizer, day Day) Summary {
	t.Helper()
	original := day
	original.Commits = append([]Commit(nil), day.Commits...)
	first, err := s.Summarize(day)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Summarize(day)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("summaries differ: %+v vs %+v", first, second)
	}
	if day.Branch != original.Branch || day.FileCount != original.FileCount || day.FileSample != original.FileSample || !sameCommits(day.Commits, original.Commits) {
		t.Fatalf("Summarize mutated day: %+v", day)
	}
	return first
}

func assertNoTicketParens(t *testing.T, got Summary) {
	t.Helper()
	if got.Ticket != "" && strings.Contains(got.Line, "("+got.Ticket+")") {
		t.Fatalf("Line = %q, ticket parentheses belong to the writer", got.Line)
	}
}
