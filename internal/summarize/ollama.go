package summarize

import "errors"

// ErrOllamaNotImplemented is returned by OllamaSummarizer in v1.
// The real model call is D.1. A caller must not treat this as an empty day.
var ErrOllamaNotImplemented = errors.New("summarize: ollama summarizer is not implemented in v1")

// OllamaSummarizer is the v1 stand-in for summarizer = "ollama".
// It satisfies Summarizer and does not call Ollama. The zero value is ready
// to use. FromConfig returns it when summarizer is "ollama". D.1 replaces Summarize.
type OllamaSummarizer struct{}

// Summarize refuses the day. It does not read commits, files, or the network,
// and it does not modify day. The summary is zero so a failed call cannot be
// written as a blank bullet.
func (OllamaSummarizer) Summarize(Day) (Summary, error) {
	return Summary{}, ErrOllamaNotImplemented
}
