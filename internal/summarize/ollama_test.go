package summarize

import (
	"errors"
	"testing"
)

// TestOllamaSummarizerIsAStub is P5.5: summarizer = "ollama" has a type
// behind it, and that type does not call a model in v1. It shares the
// Summarizer method and fails closed so a caller cannot write an empty
// bullet as if the model had summarized the day.
func TestOllamaSummarizerIsAStub(t *testing.T) {
	day := Day{
		Branch: "feature/NEXTGEN-1",
		Commits: []Commit{{
			Subject: "fix: traced the 504",
			Body:    "not sent anywhere",
			Branch:  "feature/NEXTGEN-1",
		}},
		FileCount:  2,
		FileSample: "a.go, b.go",
	}
	original := day
	original.Commits = append([]Commit(nil), day.Commits...)

	var s Summarizer = OllamaSummarizer{}
	got, err := s.Summarize(day)
	if !errors.Is(err, ErrOllamaNotImplemented) {
		t.Fatalf("err = %v, want ErrOllamaNotImplemented", err)
	}
	if got != (Summary{}) {
		t.Fatalf("Summary = %+v, want zero so the stub cannot look successful", got)
	}
	if day.Branch != original.Branch || day.FileCount != original.FileCount || day.FileSample != original.FileSample || !sameCommits(day.Commits, original.Commits) {
		t.Fatalf("Summarize mutated day: %+v", day)
	}
}

func TestOllamaSummarizerZeroValue(t *testing.T) {
	var s OllamaSummarizer
	if _, err := s.Summarize(Day{}); !errors.Is(err, ErrOllamaNotImplemented) {
		t.Fatalf("err = %v, want ErrOllamaNotImplemented", err)
	}
}
