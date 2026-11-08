package summarize

import (
	"strings"
	"testing"
)

// TestMechanicalSummarizerCondensesSubjects is P5.2: commit subjects become
// one readable line. Order is the caller's order. These cases set no
// ticket_regex and no file sample, so Ticket stays empty and the line is
// only the condensed subjects.
func TestMechanicalSummarizerCondensesSubjects(t *testing.T) {
	tests := []struct {
		name    string
		commits []Commit
		want    string
	}{
		{
			name:    "one subject",
			commits: []Commit{{Subject: "traced INT 504"}},
			want:    "traced INT 504",
		},
		{
			name:    "trims and collapses whitespace",
			commits: []Commit{{Subject: "  fixed \t  the probe path  "}},
			want:    "fixed the probe path",
		},
		{
			name:    "strips one trailing period",
			commits: []Commit{{Subject: "shipped the query."}},
			want:    "shipped the query",
		},
		{
			name:    "keeps internal periods and acronyms",
			commits: []Commit{{Subject: "bumped srv to 1.18.0"}},
			want:    "bumped srv to 1.18.0",
		},
		{
			name:    "strips a conventional prefix",
			commits: []Commit{{Subject: "fix: traced the 504"}},
			want:    "traced the 504",
		},
		{
			name:    "strips a scoped breaking prefix",
			commits: []Commit{{Subject: "feat(scan)!: honor gitignore"}},
			want:    "honor gitignore",
		},
		{
			name:    "keeps an unknown prefix",
			commits: []Commit{{Subject: "WIP: still broken"}},
			want:    "WIP: still broken",
		},
		{
			name: "joins in caller order",
			commits: []Commit{
				{Subject: "traced the 504"},
				{Subject: "ported the allow-rule"},
			},
			want: "traced the 504; ported the allow-rule",
		},
		{
			name: "drops blanks, merges, and case-duplicates",
			commits: []Commit{
				{Subject: "Fix probe"},
				{Subject: "   "},
				{Subject: "Merge branch 'main'"},
				{Subject: "fix probe"},
				{Subject: "Merge pull request #12 from hackastak/feat"},
				{Subject: "added the regression test"},
			},
			want: "Fix probe; added the regression test",
		},
		{
			name: "keeps short subjects that still fit",
			commits: []Commit{
				{Subject: "one"},
				{Subject: "two"},
				{Subject: "three"},
				{Subject: "four"},
			},
			want: "one; two; three; four",
		},
		{
			name: "condenses when another subject would exceed the line",
			commits: []Commit{
				{Subject: letters(70, 'a')},
				{Subject: letters(70, 'b')},
				{Subject: letters(70, 'c')},
				{Subject: "tail"},
			},
			want: letters(70, 'a') + "; " + letters(70, 'b') + " (+2 more)",
		},
		{
			name: "keeps a single over-long subject intact",
			commits: []Commit{
				{Subject: letters(180, 'a')},
				{Subject: "later"},
			},
			want: letters(180, 'a') + " (+1 more)",
		},
		{
			name: "nothing left after noise",
			commits: []Commit{
				{Subject: "Merge remote-tracking branch 'origin/main'"},
				{Subject: "."},
			},
			want: "",
		},
	}

	var s Summarizer = MechanicalSummarizer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := append([]Commit(nil), tt.commits...)
			got, err := s.Summarize(Day{Commits: tt.commits})
			if err != nil {
				t.Fatal(err)
			}
			if got.Line != tt.want {
				t.Fatalf("Line = %q, want %q", got.Line, tt.want)
			}
			if got.Ticket != "" {
				t.Fatalf("Ticket = %q, want empty", got.Ticket)
			}
			if !sameCommits(tt.commits, original) {
				t.Fatalf("Summarize mutated commits: %+v", tt.commits)
			}
		})
	}
}

func TestMechanicalSummarizerZeroValue(t *testing.T) {
	var s MechanicalSummarizer
	got, err := s.Summarize(Day{Commits: []Commit{{Subject: "docs: note the seam."}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Line != "note the seam" {
		t.Fatalf("Line = %q", got.Line)
	}
}

func letters(n int, c byte) string {
	return strings.Repeat(string(c), n)
}

func sameCommits(got, want []Commit) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
