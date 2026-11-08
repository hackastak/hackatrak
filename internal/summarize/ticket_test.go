package summarize

import (
	"strings"
	"testing"
)

// TestTicketExtraction is P5.4: ticket_regex is tried on the branch first,
// then the commit message. No match leaves Ticket empty so callers omit
// the parentheses. The line itself never includes them.
func TestTicketExtraction(t *testing.T) {
	const pattern = `NEXTGEN-\d+`

	tests := []struct {
		name    string
		regex   string
		day     Day
		want    string
		wantErr string
	}{
		{
			name:  "day branch wins over a commit message",
			regex: pattern,
			day: Day{
				Branch: "feature/NEXTGEN-54211-probe",
				Commits: []Commit{{
					Subject: "also mentions NEXTGEN-1",
					Body:    "body NEXTGEN-2",
					Branch:  "other/NEXTGEN-3",
				}},
			},
			want: "NEXTGEN-54211",
		},
		{
			name:  "commit branch is still a branch",
			regex: pattern,
			day: Day{
				Branch: "main",
				Commits: []Commit{{
					Subject: "message has NEXTGEN-9",
					Branch:  "bugfix/NEXTGEN-4",
				}},
			},
			want: "NEXTGEN-4",
		},
		{
			name:  "commit subject after every branch misses",
			regex: pattern,
			day: Day{
				Branch: "main",
				Commits: []Commit{
					{Subject: "no id here", Branch: "main", Body: "later body NEXTGEN-8"},
					{Subject: "fix NEXTGEN-7 timeout", Body: "NEXTGEN-6"},
				},
			},
			want: "NEXTGEN-7",
		},
		{
			name:  "commit body when the subject has no id",
			regex: pattern,
			day: Day{
				Commits: []Commit{{
					Subject: "traced the hang",
					Body:    "see NEXTGEN-15\nmore notes",
				}},
			},
			want: "NEXTGEN-15",
		},
		{
			name:  "first match in a string",
			regex: pattern,
			day:   Day{Branch: "NEXTGEN-1-and-NEXTGEN-2"},
			want:  "NEXTGEN-1",
		},
		{
			name:  "earlier commit wins over a later one",
			regex: pattern,
			day: Day{
				Commits: []Commit{
					{Subject: "NEXTGEN-3"},
					{Subject: "NEXTGEN-4"},
				},
			},
			want: "NEXTGEN-3",
		},
		{
			name:  "merge commit message still counts",
			regex: pattern,
			day: Day{
				Commits: []Commit{{Subject: "Merge branch 'feature/NEXTGEN-12'"}},
			},
			want: "NEXTGEN-12",
		},
		{
			name: "no regex leaves the ticket empty",
			day:  Day{Branch: "feature/NEXTGEN-1", Commits: []Commit{{Subject: "NEXTGEN-1"}}},
			want: "",
		},
		{
			name:  "blank regex is the same as unset",
			regex: "   ",
			day:   Day{Branch: "feature/NEXTGEN-1"},
			want:  "",
		},
		{
			name:  "no match omits the ticket",
			regex: pattern,
			day: Day{
				Branch:  "main",
				Commits: []Commit{{Subject: "traced the 504", Body: "no id"}},
			},
			want: "",
		},
		{
			name:  "case sensitive unless the pattern says otherwise",
			regex: pattern,
			day:   Day{Branch: "feature/nextgen-54211"},
			want:  "",
		},
		{
			name:  "file-only day can still carry a branch ticket",
			regex: pattern,
			day: Day{
				Branch:     "feature/NEXTGEN-20",
				FileCount:  1,
				FileSample: "main.go",
			},
			want: "NEXTGEN-20",
		},
		{
			name:    "invalid regex is an error",
			regex:   `(`,
			day:     Day{Branch: "feature/NEXTGEN-1"},
			wantErr: "ticket_regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.day
			original.Commits = append([]Commit(nil), tt.day.Commits...)
			var s Summarizer = MechanicalSummarizer{TicketRegex: tt.regex}
			got, err := s.Summarize(tt.day)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
				}
				if got != (Summary{}) {
					t.Fatalf("Summary = %+v, want zero on error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Ticket != tt.want {
				t.Fatalf("Ticket = %q, want %q", got.Ticket, tt.want)
			}
			if strings.Contains(got.Ticket, "(") || strings.Contains(got.Ticket, ")") {
				t.Fatalf("Ticket = %q, want the raw id with no parentheses", got.Ticket)
			}
			if got.Ticket != "" && strings.Contains(got.Line, "("+got.Ticket+")") {
				t.Fatalf("Line = %q, ticket parentheses belong to the writer", got.Line)
			}
			if tt.day.Branch != original.Branch || tt.day.FileCount != original.FileCount || tt.day.FileSample != original.FileSample || !sameCommits(tt.day.Commits, original.Commits) {
				t.Fatalf("Summarize mutated day: %+v", tt.day)
			}
		})
	}
}

func TestTicketExtractionKeepsTheCommitLine(t *testing.T) {
	s := MechanicalSummarizer{TicketRegex: `NEXTGEN-\d+`}
	got, err := s.Summarize(Day{
		Branch: "feature/NEXTGEN-54211",
		Commits: []Commit{{
			Subject: "fix: traced the 504.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Line != "traced the 504" {
		t.Fatalf("Line = %q, want the condensed subject", got.Line)
	}
	if got.Ticket != "NEXTGEN-54211" {
		t.Fatalf("Ticket = %q", got.Ticket)
	}
}
