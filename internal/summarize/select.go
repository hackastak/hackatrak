package summarize

import (
	"fmt"

	"github.com/hackastak/hackatrak/internal/config"
)

// FromConfig selects the Summarizer named by cfg.Summarizer.
//
// An empty name is mechanical, the same default Load applies. Mechanical
// copies ticket_regex. Ollama is the v1 stub and does not use the regex.
// Any other name is an error. cfg is not modified.
func FromConfig(cfg config.Config) (Summarizer, error) {
	switch cfg.Summarizer {
	case "", config.SummarizerMechanical:
		return MechanicalSummarizer{TicketRegex: cfg.TicketRegex}, nil
	case config.SummarizerOllama:
		return OllamaSummarizer{}, nil
	default:
		return nil, fmt.Errorf("summarize: summarizer %q is not mechanical or ollama", cfg.Summarizer)
	}
}
