package summarize

import "testing"

// stubSummarizer proves Summarizer is an interface, not a concrete type.
// P5.5's Ollama summarizer has to be able to sit behind the same method.
type stubSummarizer struct {
	line   string
	ticket string
	err    error
	saw    Day
}

func (s *stubSummarizer) Summarize(day Day) (Summary, error) {
	s.saw = day
	return Summary{Line: s.line, Ticket: s.ticket}, s.err
}

// TestSummarizerIsTheSeam is P5.1: callers depend on Summarizer, and both a
// stand-in and MechanicalSummarizer satisfy it. Summary is the clause plus
// an optional ticket, not a rendered weekly-note bullet.
func TestSummarizerIsTheSeam(t *testing.T) {
	day := Day{
		Branch: "feat/p5",
		Commits: []Commit{{
			Subject: "define the seam",
			Body:    "not the line",
			Branch:  "feat/p5",
		}},
		FileCount:  1,
		FileSample: "summarize.go",
	}

	stub := &stubSummarizer{line: "stand-in line", ticket: "NEXTGEN-1"}
	var via Summarizer = stub
	got, err := via.Summarize(day)
	if err != nil {
		t.Fatal(err)
	}
	if got.Line != "stand-in line" || got.Ticket != "NEXTGEN-1" {
		t.Fatalf("stub Summary = %+v", got)
	}
	if len(stub.saw.Commits) != 1 || stub.saw.Commits[0].Subject != "define the seam" {
		t.Fatalf("stub saw %+v, want the day passed through", stub.saw)
	}
	if stub.saw.FileCount != 1 || stub.saw.FileSample != "summarize.go" || stub.saw.Branch != "feat/p5" {
		t.Fatalf("stub saw %+v, want file and branch signals on the same input", stub.saw)
	}

	var mechanical Summarizer = MechanicalSummarizer{}
	got, err = mechanical.Summarize(day)
	if err != nil {
		t.Fatal(err)
	}
	if got.Line != "define the seam" {
		t.Fatalf("MechanicalSummarizer line = %q", got.Line)
	}
	if got.Ticket != "" {
		t.Fatalf("Ticket = %q, want empty until ticket extraction", got.Ticket)
	}
}
