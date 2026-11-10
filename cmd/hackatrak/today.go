package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/query"
	"github.com/hackastak/hackatrak/internal/summarize"
)

func newTodayCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Print today's per-project work",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToday(cmd, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	return cmd
}

func runToday(cmd *cobra.Command, asJSON bool) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	q, err := query.New(state.db, nil)
	if err != nil {
		return fmt.Errorf("today: %w", err)
	}
	days, err := q.TodaysWork()
	if err != nil {
		return fmt.Errorf("today: %w", err)
	}
	summarizer, err := summarize.FromConfig(state.cfg)
	if err != nil {
		return fmt.Errorf("today: %w", err)
	}
	if asJSON {
		date := localDate(time.Now())
		if worked := workedDays(days); len(worked) > 0 {
			date = formatQueryDate(worked[0].Day)
		}
		doc, err := reviewFrom(days, summarizer, "day", date, "", "today")
		if err != nil {
			return err
		}
		return writeReviewJSON(cmd.OutOrStdout(), doc)
	}
	return writeWorkTable(cmd.OutOrStdout(), days, summarizer, "today")
}

// writeWorkTable prints a human table of project-days.
// An empty set prints "<label>: no work". Rows stay in query order.
// The summarizer supplies the WORK cell only; identity columns come from query.
func writeWorkTable(w io.Writer, days []query.ProjectDay, summarizer summarize.Summarizer, label string) error {
	worked := workedDays(days)
	if len(worked) == 0 {
		_, err := fmt.Fprintf(w, "%s: no work\n", label)
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n\n", formatQueryDate(worked[0].Day)); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if err := writeDayTable(tw, worked, summarizer, label); err != nil {
		return err
	}
	return tw.Flush()
}

func writeDayTable(w io.Writer, days []query.ProjectDay, summarizer summarize.Summarizer, label string) error {
	if _, err := fmt.Fprintln(w, "PROJECT\tBUCKET\tSOURCE\tBRANCH\tTICKET\tWORK"); err != nil {
		return err
	}
	for _, day := range days {
		row, err := formatWorkRow(summarizer, day, label)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, row); err != nil {
			return err
		}
	}
	return nil
}

func formatWorkRow(summarizer summarize.Summarizer, day query.ProjectDay, label string) (string, error) {
	line, err := workLine(summarizer, day)
	if err != nil {
		return "", fmt.Errorf("%s: summarize %s: %w", label, projectCell(day), err)
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
		projectCell(day),
		cell(day.Bucket),
		cell(string(day.Source)),
		cell(day.Branch),
		cell(day.Ticket),
		cell(line),
	), nil
}

func workedDays(days []query.ProjectDay) []query.ProjectDay {
	if len(days) == 0 {
		return nil
	}
	worked := make([]query.ProjectDay, 0, len(days))
	for _, day := range days {
		if !day.Worked {
			continue
		}
		worked = append(worked, day)
	}
	return worked
}

func workLine(summarizer summarize.Summarizer, day query.ProjectDay) (string, error) {
	if summarizer == nil {
		return "", fmt.Errorf("summarizer is required")
	}
	summary, err := summarizer.Summarize(summaryInput(day))
	if err != nil {
		return "", err
	}
	return summary.Line, nil
}

func summaryInput(day query.ProjectDay) summarize.Day {
	commits := make([]summarize.Commit, len(day.Commits))
	for i, commit := range day.Commits {
		commits[i] = summarize.Commit{
			Subject: commit.Subject,
			Body:    commit.Body,
			Branch:  commit.Branch,
		}
	}
	return summarize.Day{
		Branch:     day.Branch,
		Commits:    commits,
		FileCount:  day.FileCount,
		FileSample: day.FileSample,
	}
}

func projectCell(day query.ProjectDay) string {
	if strings.TrimSpace(day.Slug) != "" {
		return cell(day.Slug)
	}
	return cell(day.Name)
}

func formatQueryDate(day query.Date) string {
	return fmt.Sprintf("%04d-%02d-%02d", day.Year, day.Month, day.Day)
}

func cell(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "-"
	}
	return value
}
