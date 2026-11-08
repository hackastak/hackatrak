// Package summarize builds the one-line "what was done" text.
//
// The line is the summary clause only. The vault writer adds the wikilink,
// the optional ticket parentheses, and the marker. Query stays data; this
// package is where that data becomes a sentence.
package summarize

// Commit is one commit that can contribute to the line.
// Subject is the raw git subject. Body and Branch are available to later
// summarizers; mechanical condensation reads Subject only.
type Commit struct {
	Subject string
	Body    string
	Branch  string
}

// Day is one project-day's raw signals.
//
// Commits are in the order the caller recorded them, usually earliest first.
// Branch is the day's branch, not a formatted prefix. FileCount and FileSample
// are the presence fallback; a later summarizer reads them when Commits is empty.
type Day struct {
	Branch     string
	Commits    []Commit
	FileCount  int
	FileSample string
}

// Summary is the "what was done" clause, not a weekly-note bullet.
//
// Line has no project wikilink and no ticket parentheses. Ticket is the raw
// id when a summarizer extracted one, and empty when it did not. Callers
// omit the parentheses for an empty Ticket.
type Summary struct {
	Line   string
	Ticket string
}

// Summarizer turns one project-day into Summary.
// Mechanical condensation and a later model summarizer share this method.
// A summarizer must not modify Day.
type Summarizer interface {
	Summarize(Day) (Summary, error)
}
