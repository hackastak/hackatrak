package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hackastak/hackatrak/internal/query"
	"github.com/hackastak/hackatrak/internal/summarize"
)

// reviewDoc is the --json shape for today and summary.
// Projects is never null. Day rows use date; week rows use week.
type reviewDoc struct {
	Period       string      `json:"period"`
	Date         string      `json:"date,omitempty"`
	Week         string      `json:"week,omitempty"`
	Projects     []reviewRow `json:"projects"`
	ProjectCount int         `json:"project_count"`
	DayCount     int         `json:"day_count"`
}

type reviewRow struct {
	Day     string `json:"day"`
	Project string `json:"project"`
	Bucket  string `json:"bucket"`
	Source  string `json:"source"`
	Branch  string `json:"branch"`
	Ticket  string `json:"ticket"`
	Work    string `json:"work"`
}

func reviewFrom(days []query.ProjectDay, summarizer summarize.Summarizer, period, date, week, label string) (reviewDoc, error) {
	worked := workedDays(days)
	rows := make([]reviewRow, 0, len(worked))
	for _, day := range worked {
		line, err := workLine(summarizer, day)
		if err != nil {
			return reviewDoc{}, fmt.Errorf("%s: summarize %s: %w", label, projectCell(day), err)
		}
		rows = append(rows, reviewRow{
			Day:     formatQueryDate(day.Day),
			Project: rawProject(day),
			Bucket:  day.Bucket,
			Source:  string(day.Source),
			Branch:  day.Branch,
			Ticket:  day.Ticket,
			Work:    line,
		})
	}
	return reviewDoc{
		Period:       period,
		Date:         date,
		Week:         week,
		Projects:     rows,
		ProjectCount: uniqueProjects(worked),
		DayCount:     len(groupByDay(worked)),
	}, nil
}

func rawProject(day query.ProjectDay) string {
	if day.Slug != "" {
		return day.Slug
	}
	return day.Name
}

func writeReviewJSON(w io.Writer, doc reviewDoc) error {
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = w.Write(encoded)
	return err
}

func localDate(now time.Time) string {
	return now.In(time.Local).Format("2006-01-02")
}
