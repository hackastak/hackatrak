package query

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hackastak/hackatrak/internal/store"
)

// TodaysWork returns project-days for the local calendar day of the clock.
// The day boundary is q's location, not UTC. No rows returns nil.
// Order is slug. Name, root, and bucket stay empty until a project row exists.
func (q *Query) TodaysWork() ([]ProjectDay, error) {
	if err := q.ready(); err != nil {
		return nil, fmt.Errorf("today's work: %w", err)
	}
	now := time.Now
	if q.now != nil {
		now = q.now
	}
	loc := q.loc
	if loc == nil {
		loc = time.Local
	}
	day := now().In(loc).Format("2006-01-02")
	return q.workBetween(day, day, "today's work")
}

// WeekWork returns project-days in the Monday-start ISO week.
// The week can cross a calendar year. Days outside that week are omitted.
// Order is day, then slug. An unknown week is an error. No rows returns nil.
func (q *Query) WeekWork(iso ISOWeek) ([]ProjectDay, error) {
	if err := q.ready(); err != nil {
		return nil, fmt.Errorf("week work: %w", err)
	}
	from, to, err := isoBounds(iso)
	if err != nil {
		return nil, fmt.Errorf("week work: %w", err)
	}
	return q.workBetween(from, to, "week work")
}

func (q *Query) ready() error {
	if q == nil || q.db == nil {
		return fmt.Errorf("query is required")
	}
	return nil
}

func (q *Query) workBetween(from, to, op string) ([]ProjectDay, error) {
	rows, err := q.db.DayPresences(from, to)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	days := make([]ProjectDay, 0, len(rows))
	for _, row := range rows {
		day, err := q.projectDay(row)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		days = append(days, day)
	}
	sort.SliceStable(days, func(i, j int) bool {
		if days[i].Day != days[j].Day {
			return dateBefore(days[i].Day, days[j].Day)
		}
		return days[i].Slug < days[j].Slug
	})
	return days, nil
}

func (q *Query) projectDay(row store.DayPresence) (ProjectDay, error) {
	day, err := dateFrom(row.Day)
	if err != nil {
		return ProjectDay{}, err
	}
	source, err := sourceFrom(row.Source)
	if err != nil {
		return ProjectDay{}, err
	}
	commits, err := q.commits(row.CommitHashes)
	if err != nil {
		return ProjectDay{}, err
	}
	return ProjectDay{
		Slug:       row.ProjectSlug,
		Day:        day,
		Worked:     row.Worked,
		Source:     source,
		Branch:     row.Branch,
		Ticket:     row.Ticket,
		Commits:    commits,
		FileCount:  row.FileCount,
		FileSample: row.FileSample,
	}, nil
}

func (q *Query) commits(hashes []string) ([]Commit, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	commits := make([]Commit, 0, len(hashes))
	for _, hash := range hashes {
		event, err := q.db.EventByHash(hash)
		if errors.Is(err, store.ErrNotFound) {
			commits = append(commits, Commit{Hash: hash})
			continue
		}
		if err != nil {
			return nil, err
		}
		commits = append(commits, Commit{
			Hash:    hash,
			Branch:  event.Branch,
			Subject: event.Subject,
			Body:    event.Body,
		})
	}
	return commits, nil
}

func sourceFrom(source store.PresenceSource) (Source, error) {
	switch source {
	case store.PresenceCommit:
		return SourceCommit, nil
	case store.PresenceFile:
		return SourceFile, nil
	default:
		return "", fmt.Errorf("source %q is not supported", source)
	}
}

func dateFrom(day string) (Date, error) {
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		return Date{}, fmt.Errorf("day %q: %w", day, err)
	}
	return Date{Year: parsed.Year(), Month: parsed.Month(), Day: parsed.Day()}, nil
}

func dateBefore(a, b Date) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}

// isoBounds returns the Monday and Sunday of an ISO week as YYYY-MM-DD.
// The calculation uses UTC dates so a location cannot shift the calendar day.
func isoBounds(iso ISOWeek) (string, string, error) {
	if iso.Year < 1 || iso.Week < 1 || iso.Week > 53 {
		return "", "", fmt.Errorf("iso week %d-W%02d is invalid", iso.Year, iso.Week)
	}
	jan4 := time.Date(iso.Year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := jan4.AddDate(0, 0, -(weekday-1)+(iso.Week-1)*7)
	year, week := monday.ISOWeek()
	if year != iso.Year || week != iso.Week {
		return "", "", fmt.Errorf("iso week %d-W%02d does not exist", iso.Year, iso.Week)
	}
	sunday := monday.AddDate(0, 0, 6)
	return monday.Format("2006-01-02"), sunday.Format("2006-01-02"), nil
}
