package vault

import (
	"path/filepath"
	"testing"
	"time"
)

func TestISOWeekMondayStart(t *testing.T) {
	chicago := mustLocation(t, "America/Chicago")
	utc := time.UTC

	tests := []struct {
		name    string
		day     time.Time
		year    int
		week    int
		monday  string
		relPath string
	}{
		{
			name:    "thursday opening 2026 is W01 in the new week-year",
			day:     time.Date(2026, 1, 1, 18, 30, 0, 0, chicago),
			year:    2026,
			week:    1,
			monday:  "2025-12-29",
			relPath: "_Weekly/2026/2026-W01.md",
		},
		{
			name:    "sunday still belongs to the week that started monday",
			day:     time.Date(2026, 1, 4, 23, 59, 0, 0, chicago),
			year:    2026,
			week:    1,
			monday:  "2025-12-29",
			relPath: "_Weekly/2026/2026-W01.md",
		},
		{
			name:    "monday rolls to the next ISO week",
			day:     time.Date(2026, 1, 5, 0, 0, 0, 0, chicago),
			year:    2026,
			week:    2,
			monday:  "2026-01-05",
			relPath: "_Weekly/2026/2026-W02.md",
		},
		{
			name:    "week number is zero padded",
			day:     time.Date(2026, 2, 23, 12, 0, 0, 0, chicago),
			year:    2026,
			week:    9,
			monday:  "2026-02-23",
			relPath: "_Weekly/2026/2026-W09.md",
		},
		{
			name:    "calendar year before the ISO week-year",
			day:     time.Date(2025, 12, 29, 9, 0, 0, 0, utc),
			year:    2026,
			week:    1,
			monday:  "2025-12-29",
			relPath: "_Weekly/2026/2026-W01.md",
		},
		{
			name:    "last sunday of 2025 stays in 2025-W52",
			day:     time.Date(2025, 12, 28, 12, 0, 0, 0, utc),
			year:    2025,
			week:    52,
			monday:  "2025-12-22",
			relPath: "_Weekly/2025/2025-W52.md",
		},
		{
			name:    "week 53 lives under the previous week-year",
			day:     time.Date(2021, 1, 1, 0, 30, 0, 0, chicago),
			year:    2020,
			week:    53,
			monday:  "2020-12-28",
			relPath: "_Weekly/2020/2020-W53.md",
		},
		{
			name:    "first monday after week 53 is W01",
			day:     time.Date(2021, 1, 4, 8, 0, 0, 0, chicago),
			year:    2021,
			week:    1,
			monday:  "2021-01-04",
			relPath: "_Weekly/2021/2021-W01.md",
		},
		{
			name:    "monday of a week-year that starts in december",
			day:     time.Date(2019, 12, 30, 15, 0, 0, 0, utc),
			year:    2020,
			week:    1,
			monday:  "2019-12-30",
			relPath: "_Weekly/2020/2020-W01.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ISOWeek(tt.day)
			if got.Year != tt.year || got.Week != tt.week {
				t.Fatalf("ISOWeek(%s) = %d-W%02d, want %d-W%02d", tt.day.Format(time.RFC3339), got.Year, got.Week, tt.year, tt.week)
			}
			monday := WeekMonday(tt.day)
			if monday.Format("2006-01-02") != tt.monday {
				t.Fatalf("WeekMonday = %s, want %s", monday.Format("2006-01-02"), tt.monday)
			}
			if monday.Hour() != 0 || monday.Minute() != 0 || monday.Location().String() != tt.day.Location().String() {
				t.Fatalf("WeekMonday = %s, want local midnight", monday)
			}
			path, err := NotePath("/vault", tt.day)
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join("/vault", filepath.FromSlash(tt.relPath))
			if path != want {
				t.Fatalf("NotePath = %s, want %s", path, want)
			}
		})
	}
}

func TestResolveNotePath(t *testing.T) {
	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	t.Run("empty glob uses the default week path", func(t *testing.T) {
		path, err := ResolveNotePath("/vault", "  ", day)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("/vault", filepath.FromSlash("_Weekly/2026/2026-W02.md"))
		if path != want {
			t.Fatalf("path = %s, want %s", path, want)
		}
	})

	t.Run("placeholders expand to the ISO week-year", func(t *testing.T) {
		path, err := ResolveNotePath("/vault", "notes/{YYYY}/week-{ww}.md", day)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("/vault", "notes", "2026", "week-02.md")
		if path != want {
			t.Fatalf("path = %s, want %s", path, want)
		}
	})

	t.Run("empty vault path", func(t *testing.T) {
		if _, err := NotePath("  ", day); err == nil {
			t.Fatal("expected error for empty vault path")
		}
	})

	t.Run("absolute glob is rejected", func(t *testing.T) {
		if _, err := ResolveNotePath("/vault", "/tmp/{YYYY}-W{ww}.md", day); err == nil {
			t.Fatal("expected error for absolute glob")
		}
	})

	t.Run("parent segment is rejected", func(t *testing.T) {
		if _, err := ResolveNotePath("/vault", "../{YYYY}-W{ww}.md", day); err == nil {
			t.Fatal("expected error for glob that escapes the vault")
		}
	})
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
