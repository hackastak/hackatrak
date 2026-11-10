package main

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/query"
	"github.com/hackastak/hackatrak/internal/summarize"
)

func newSummaryCmd() *cobra.Command {
	var week bool
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Roll up per-project work for today or the current week",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSummary(cmd, week, asJSON)
		},
	}
	cmd.Flags().BoolVar(&week, "week", false, "roll up the current ISO week instead of today")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	return cmd
}

func runSummary(cmd *cobra.Command, week, asJSON bool) error {
	state, err := appFrom(cmd)
	if err != nil {
		return err
	}
	q, err := query.New(state.db, nil)
	if err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	days, heading, err := summaryDays(q, week)
	if err != nil {
		return err
	}
	summarizer, err := summarize.FromConfig(state.cfg)
	if err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	if asJSON {
		period := "day"
		date := localDate(time.Now())
		weekLabel := ""
		if week {
			period = "week"
			date = ""
			weekLabel = heading
		} else if worked := workedDays(days); len(worked) > 0 {
			date = formatQueryDate(worked[0].Day)
		}
		doc, err := reviewFrom(days, summarizer, period, date, weekLabel, "summary")
		if err != nil {
			return err
		}
		return writeReviewJSON(cmd.OutOrStdout(), doc)
	}
	return writeRollup(cmd.OutOrStdout(), days, summarizer, heading)
}

func summaryDays(q *query.Query, week bool) ([]query.ProjectDay, string, error) {
	if !week {
		days, err := q.TodaysWork()
		if err != nil {
			return nil, "", fmt.Errorf("summary: %w", err)
		}
		return days, "", nil
	}
	now := time.Now().In(time.Local)
	year, weekNum := now.ISOWeek()
	days, err := q.WeekWork(query.ISOWeek{Year: year, Week: weekNum})
	if err != nil {
		return nil, "", fmt.Errorf("summary: %w", err)
	}
	return days, fmt.Sprintf("%04d-W%02d", year, weekNum), nil
}

// writeRollup prints project-days grouped by calendar day, then a count line.
// heading is the week label; empty means a single-day rollup. Query order is kept.
func writeRollup(w io.Writer, days []query.ProjectDay, summarizer summarize.Summarizer, heading string) error {
	worked := workedDays(days)
	if len(worked) == 0 {
		_, err := fmt.Fprintln(w, "summary: no work")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if heading != "" {
		if _, err := fmt.Fprintf(tw, "%s\n\n", heading); err != nil {
			return err
		}
	}
	groups := groupByDay(worked)
	for i, group := range groups {
		if i > 0 {
			if _, err := fmt.Fprintln(tw); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(tw, "%s\n\n", formatQueryDate(group[0].Day)); err != nil {
			return err
		}
		if err := writeDayTable(tw, group, summarizer, "summary"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(tw, "\n%s\n", rollupLine(worked, len(groups), heading != "")); err != nil {
		return err
	}
	return tw.Flush()
}

func groupByDay(days []query.ProjectDay) [][]query.ProjectDay {
	if len(days) == 0 {
		return nil
	}
	groups := make([][]query.ProjectDay, 0, 1)
	for _, day := range days {
		if len(groups) == 0 || groups[len(groups)-1][0].Day != day.Day {
			groups = append(groups, nil)
		}
		last := len(groups) - 1
		groups[last] = append(groups[last], day)
	}
	return groups
}

func rollupLine(days []query.ProjectDay, dayCount int, week bool) string {
	projects := plural(uniqueProjects(days), "project")
	if !week {
		return projects
	}
	return projects + " over " + plural(dayCount, "day")
}

func uniqueProjects(days []query.ProjectDay) int {
	seen := make(map[string]struct{}, len(days))
	for _, day := range days {
		key := day.Slug
		if key == "" {
			key = day.Name
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
