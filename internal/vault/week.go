package vault

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
)

// Week is an ISO-8601 week. Weeks start on Monday. Year is the week-year,
// which can differ from the calendar year at the December/January boundary.
type Week struct {
	Year int
	Week int
}

// ISOWeek returns the Monday-start ISO week of day, using day's location.
func ISOWeek(day time.Time) Week {
	year, week := day.ISOWeek()
	return Week{Year: year, Week: week}
}

// WeekMonday is midnight on the Monday of day's ISO week, in day's location.
func WeekMonday(day time.Time) time.Time {
	year, month, date := day.Date()
	midnight := time.Date(year, month, date, 0, 0, 0, 0, day.Location())
	daysSinceMonday := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -daysSinceMonday)
}

// NotePath resolves the weekly note under vaultPath using the default glob
// `_Weekly/{YYYY}/{YYYY}-W{ww}.md`. YYYY is the ISO week-year.
func NotePath(vaultPath string, day time.Time) (string, error) {
	return ResolveNotePath(vaultPath, config.DefaultWeeklyGlob, day)
}

// ResolveNotePath fills {YYYY} and {ww} in glob and joins it to vaultPath.
// glob must be relative and must not contain "..". An empty glob uses the default.
func ResolveNotePath(vaultPath, glob string, day time.Time) (string, error) {
	vault := strings.TrimSpace(vaultPath)
	if vault == "" {
		return "", fmt.Errorf("vault: vault path is empty")
	}
	pattern := strings.TrimSpace(glob)
	if pattern == "" {
		pattern = config.DefaultWeeklyGlob
	}
	if filepath.IsAbs(pattern) || strings.Contains(pattern, "..") {
		return "", fmt.Errorf("vault: weekly glob must be a relative path under the vault")
	}
	week := ISOWeek(day)
	rel := strings.NewReplacer(
		"{YYYY}", fmt.Sprintf("%04d", week.Year),
		"{ww}", fmt.Sprintf("%02d", week.Week),
	).Replace(pattern)
	return filepath.Join(vault, filepath.FromSlash(rel)), nil
}
