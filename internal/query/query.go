// Package query is the typed read API for today and week views.
//
// The types here are data only. They do not render bullets, weekday names,
// or tables. Presentation formats them.
package query

import (
	"fmt"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// Source is why a project-day counts as worked.
// The values match store.PresenceCommit and store.PresenceFile.
type Source string

const (
	// SourceCommit means the day has at least one commit.
	SourceCommit Source = "commit"
	// SourceFile means the day has file touches and no commits.
	SourceFile Source = "file"
)

// Date is a local calendar day. It has no clock and no formatted text.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// ISOWeek is an ISO-8601 week, Monday-start.
// Year is the ISO week-year, which can differ from the calendar year at the boundary.
// Week is 1–53. This type does not format itself as 2026-W45.
type ISOWeek struct {
	Year int
	Week int
}

// Commit is one commit that counted toward a project-day.
// Subject and Body are the raw git text, not a summary sentence.
type Commit struct {
	Hash    string
	Branch  string
	Subject string
	Body    string
}

// ProjectDay is one project on one local calendar day.
//
// Slug is the wikilink target. Name, Root, and Bucket come from the project
// record when one exists; they may be empty when only day-presence is stored.
// Ticket is the raw id, with no parentheses. FileSample is the scan sample,
// not a list rendered here. Commits are in the order the day recorded them.
type ProjectDay struct {
	Slug       string
	Name       string
	Root       string
	Bucket     string
	Day        Date
	Worked     bool
	Source     Source
	Branch     string
	Ticket     string
	Commits    []Commit
	FileCount  int
	FileSample string
}

// Query reads project-days. It holds the store and the local day boundary.
// now is the clock for TodaysWork. Tests in this package may replace it.
type Query struct {
	db  *store.DB
	loc *time.Location
	now func() time.Time
}

// New returns a Query for db. A nil loc uses time.Local, the same day
// boundary as derive and the file scan. A nil db is an error.
func New(db *store.DB, loc *time.Location) (*Query, error) {
	if db == nil {
		return nil, fmt.Errorf("query: database is required")
	}
	if loc == nil {
		loc = time.Local
	}
	return &Query{db: db, loc: loc, now: time.Now}, nil
}
