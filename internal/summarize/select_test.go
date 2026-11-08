package summarize

import (
	"errors"
	"strings"
	"testing"

	"github.com/hackastak/hackatrak/internal/config"
)

// TestFromConfig is P5.6: the config summarizer field chooses the
// implementation. Mechanical receives ticket_regex. Ollama is the v1 stub.
// An unknown value is an error, not a silent fallback.
func TestFromConfig(t *testing.T) {
	day := Day{
		Branch:  "feature/NEXTGEN-9",
		Commits: []Commit{{Subject: "fix: traced the 504"}},
	}

	t.Run("mechanical", func(t *testing.T) {
		got, err := FromConfig(config.Config{
			Summarizer:  config.SummarizerMechanical,
			TicketRegex: `NEXTGEN-\d+`,
		})
		if err != nil {
			t.Fatal(err)
		}
		mech, ok := got.(MechanicalSummarizer)
		if !ok {
			t.Fatalf("got %T, want MechanicalSummarizer", got)
		}
		if mech.TicketRegex != `NEXTGEN-\d+` {
			t.Fatalf("TicketRegex = %q", mech.TicketRegex)
		}
		summary, err := got.Summarize(day)
		if err != nil {
			t.Fatal(err)
		}
		if summary.Line != "traced the 504" || summary.Ticket != "NEXTGEN-9" {
			t.Fatalf("Summary = %+v", summary)
		}
	})

	t.Run("empty is mechanical", func(t *testing.T) {
		got, err := FromConfig(config.Config{TicketRegex: `NEXTGEN-\d+`})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(MechanicalSummarizer); !ok {
			t.Fatalf("got %T, want MechanicalSummarizer", got)
		}
		summary, err := got.Summarize(day)
		if err != nil {
			t.Fatal(err)
		}
		if summary.Ticket != "NEXTGEN-9" {
			t.Fatalf("Ticket = %q, want the config regex applied", summary.Ticket)
		}
	})

	t.Run("ollama", func(t *testing.T) {
		got, err := FromConfig(config.Config{
			Summarizer:  config.SummarizerOllama,
			TicketRegex: `NEXTGEN-\d+`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(OllamaSummarizer); !ok {
			t.Fatalf("got %T, want OllamaSummarizer", got)
		}
		summary, err := got.Summarize(day)
		if !errors.Is(err, ErrOllamaNotImplemented) {
			t.Fatalf("err = %v, want ErrOllamaNotImplemented", err)
		}
		if summary != (Summary{}) {
			t.Fatalf("Summary = %+v, want zero", summary)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		got, err := FromConfig(config.Config{Summarizer: "gpt"})
		if err == nil || !strings.Contains(err.Error(), "summarizer") {
			t.Fatalf("err = %v, want it to mention summarizer", err)
		}
		if got != nil {
			t.Fatalf("got %#v, want nil", got)
		}
	})
}
