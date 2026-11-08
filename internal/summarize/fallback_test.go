package summarize

import "testing"

// TestFileFallbackText is P5.3: a day with no usable commit subject says
// which files were touched. Commit subjects still win when any survive.
func TestFileFallbackText(t *testing.T) {
	tests := []struct {
		name string
		day  Day
		want string
	}{
		{
			name: "sample shorter than the count gets an ellipsis",
			day: Day{
				FileCount:  4,
				FileSample: "a.go, b.go, c.go",
			},
			want: "touched 4 files (a.go, b.go, c.go, …)",
		},
		{
			name: "sample that lists every file has no ellipsis",
			day: Day{
				FileCount:  2,
				FileSample: "README.md, main.go",
			},
			want: "touched 2 files (README.md, main.go)",
		},
		{
			name: "one file is singular",
			day: Day{
				FileCount:  1,
				FileSample: "probe.go",
			},
			want: "touched 1 file (probe.go)",
		},
		{
			name: "count with an empty sample omits the parentheses",
			day:  Day{FileCount: 3},
			want: "touched 3 files",
		},
		{
			name: "trims sample names and ignores blanks",
			day: Day{
				FileCount:  2,
				FileSample: " a.go ,  , b.go ",
			},
			want: "touched 2 files (a.go, b.go)",
		},
		{
			name: "does not list more names than the count",
			day: Day{
				FileCount:  1,
				FileSample: "a.go, b.go",
			},
			want: "touched 1 file (a.go)",
		},
		{
			name: "no files and no commits",
			day:  Day{},
			want: "",
		},
		{
			name: "negative count is not a file day",
			day:  Day{FileCount: -1, FileSample: "a.go"},
			want: "",
		},
		{
			name: "commit subjects win over a file sample",
			day: Day{
				Commits:    []Commit{{Subject: "traced the 504"}},
				FileCount:  3,
				FileSample: "a.go, b.go",
			},
			want: "traced the 504",
		},
		{
			name: "noise-only commits fall through to the file text",
			day: Day{
				Commits:    []Commit{{Subject: "Merge branch 'main'"}, {Subject: "."}},
				FileCount:  2,
				FileSample: "a.go, b.go",
			},
			want: "touched 2 files (a.go, b.go)",
		},
	}

	var s Summarizer = MechanicalSummarizer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.day
			original.Commits = append([]Commit(nil), tt.day.Commits...)
			got, err := s.Summarize(tt.day)
			if err != nil {
				t.Fatal(err)
			}
			if got.Line != tt.want {
				t.Fatalf("Line = %q, want %q", got.Line, tt.want)
			}
			if got.Ticket != "" {
				t.Fatalf("Ticket = %q, want empty without a ticket_regex", got.Ticket)
			}
			if tt.day.FileCount != original.FileCount || tt.day.FileSample != original.FileSample || tt.day.Branch != original.Branch || !sameCommits(tt.day.Commits, original.Commits) {
				t.Fatalf("Summarize mutated day: %+v", tt.day)
			}
		})
	}
}
