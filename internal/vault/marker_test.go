package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMarkerFormat(t *testing.T) {
	got, err := Marker("staksmith")
	if err != nil {
		t.Fatal(err)
	}
	if got != "<!-- ht:staksmith -->" {
		t.Fatalf("Marker = %q", got)
	}

	got, err = Marker("supply-chain-monitor")
	if err != nil {
		t.Fatal(err)
	}
	if got != "<!-- ht:supply-chain-monitor -->" {
		t.Fatalf("Marker = %q", got)
	}

	got, err = Marker("My Repo")
	if err != nil {
		t.Fatal(err)
	}
	if got != "<!-- ht:My Repo -->" {
		t.Fatalf("Marker = %q", got)
	}

	line := "- [[staksmith]] (NEXTGEN-123): reworked docs flow <!-- ht:staksmith -->"
	slug, ok := MarkerSlug(line)
	if !ok || slug != "staksmith" {
		t.Fatalf("MarkerSlug = %q, %v", slug, ok)
	}

	rejects := []string{"", "  ", " staksmith", "staksmith ", "foo--bar", "a>b", "a\nb", "<!-- ht:x -->"}
	for _, slug := range rejects {
		if _, err := Marker(slug); !errors.Is(err, ErrInvalidSlug) {
			t.Fatalf("Marker(%q) err = %v, want ErrInvalidSlug", slug, err)
		}
	}
}

func TestMarkerSlugMatching(t *testing.T) {
	tests := []struct {
		line string
		slug string
		ok   bool
	}{
		{line: "- [[staksmith]]: work <!-- ht:staksmith -->", slug: "staksmith", ok: true},
		{line: "- [[staksmith]]: work <!-- ht:staksmith -->  ", slug: "staksmith", ok: true},
		{line: "\t- nested <!-- ht:My Repo -->", slug: "My Repo", ok: true},
		{line: "- [[staksmith]]: work", slug: "", ok: false},
		{line: "- looks marked <!-- ht:staksmith-->", slug: "", ok: false},
		{line: "- looks marked <!--ht:staksmith -->", slug: "", ok: false},
		{line: "- looks marked <!-- HT:staksmith -->", slug: "", ok: false},
		{line: "- looks marked <!-- ht: -->", slug: "", ok: false},
		{line: "- prefix <!-- ht:hack -->", slug: "hack", ok: true},
		{line: "- longer <!-- ht:hackatrak -->", slug: "hackatrak", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			slug, ok := MarkerSlug(tt.line)
			if ok != tt.ok || slug != tt.slug {
				t.Fatalf("MarkerSlug = %q, %v; want %q, %v", slug, ok, tt.slug, tt.ok)
			}
		})
	}

	slug, ok := MarkerSlug("- first <!-- ht:hack --> then <!-- ht:hackatrak -->")
	if !ok || slug != "hack" {
		t.Fatalf("first marker = %q, %v", slug, ok)
	}
	slug, ok = MarkerSlug("- prefix <!-- ht:hackatrak -->")
	if !ok || slug == "hack" {
		t.Fatalf("hackatrak marker = %q, %v", slug, ok)
	}
}

func TestFindMarkedLineUnderToday(t *testing.T) {
	note := string(readFixture(t, "2026-W45.md"))
	chicago := mustLocation(t, "America/Chicago")

	tests := []struct {
		name    string
		bucket  string
		slug    string
		day     time.Time
		want    string
		wantIdx bool
	}{
		{
			name:    "sap monday marker",
			bucket:  "SAP Business Network & ANT",
			slug:    "supply-chain-monitor",
			day:     time.Date(2026, 11, 2, 15, 0, 0, 0, chicago),
			want:    "- [[supply-chain-monitor]] (NEXTGEN-60297): root-caused the shared batch failure <!-- ht:supply-chain-monitor -->",
			wantIdx: true,
		},
		{
			name:    "hackastak tuesday uses the first marked line only",
			bucket:  "Hackastak",
			slug:    "hackatrak",
			day:     time.Date(2026, 11, 3, 11, 0, 0, 0, chicago),
			want:    "- Hackatrak: added git log parsing <!-- ht:hackatrak -->",
			wantIdx: true,
		},
		{
			name:    "case sensitive slug",
			bucket:  "Hackastak",
			slug:    "Hackatrak",
			day:     time.Date(2026, 11, 5, 11, 0, 0, 0, chicago),
			want:    "- Hackatrak: added the file mtime scan <!-- ht:Hackatrak -->",
			wantIdx: true,
		},
		{
			name:   "same slug on another weekday is not today",
			bucket: "Hackastak",
			slug:   "hackatrak",
			day:    time.Date(2026, 11, 2, 11, 0, 0, 0, chicago),
		},
		{
			name:   "checklist marker above the journal is not under monday",
			bucket: "Hackastak",
			slug:   "hackatrak",
			day:    time.Date(2026, 11, 2, 11, 0, 0, 0, chicago),
		},
		{
			name:   "other bucket marker does not leak",
			bucket: "Hackastak",
			slug:   "supply-chain-monitor",
			day:    time.Date(2026, 11, 2, 11, 0, 0, 0, chicago),
		},
		{
			name:   "unmarked hand-written day",
			bucket: "SAP Business Network & ANT",
			slug:   "supply-chain-monitor",
			day:    time.Date(2026, 11, 3, 11, 0, 0, 0, chicago),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := FindMarkedLine(note, tt.bucket, tt.slug, tt.day)
			if err != nil {
				t.Fatal(err)
			}
			if !tt.wantIdx {
				if ok {
					t.Fatalf("found %q at %d, want none", got.Text, got.Index)
				}
				return
			}
			if !ok {
				t.Fatal("marked line not found")
			}
			if got.Text != tt.want || got.Slug != tt.slug {
				t.Fatalf("line = %#v, want text %q slug %q", got, tt.want, tt.slug)
			}
			lines := strings.Split(note, "\n")
			if lines[got.Index] != got.Text {
				t.Fatalf("index %d = %q, want %q", got.Index, lines[got.Index], got.Text)
			}
			again, ok, err := FindMarkedLine(note, tt.bucket, tt.slug, tt.day)
			if err != nil || !ok || again != got {
				t.Fatalf("second find = %#v, %v, %v", again, ok, err)
			}
		})
	}
}

func TestFindMarkedLineErrors(t *testing.T) {
	note := "# Hackastak\n\n#### Daily Journal\nMonday\n- hand written\n"
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

	_, ok, err := FindMarkedLine(note, "Hackastak", "hackatrak", day)
	if err != nil || ok {
		t.Fatalf("missing marker = %v, %v; want false, nil", ok, err)
	}

	_, _, err = FindMarkedLine(note, "Missing", "hackatrak", day)
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("err = %v, want ErrBucketNotFound", err)
	}

	_, _, err = FindMarkedLine(note, "Hackastak", "hackatrak", day.AddDate(0, 0, 1))
	if !errors.Is(err, ErrWeekdayNotFound) {
		t.Fatalf("err = %v, want ErrWeekdayNotFound", err)
	}

	_, ok, err = FindMarkedLine(note, "Hackastak", "foo--bar", day)
	if !errors.Is(err, ErrInvalidSlug) || ok {
		t.Fatalf("invalid slug = %v, %v", ok, err)
	}
}

func TestFindMarkedLineNestedAndTrailingSpace(t *testing.T) {
	marker, err := Marker("My Repo")
	if err != nil {
		t.Fatal(err)
	}
	note := "# Hackastak\n\n#### Daily Journal\nSunday\n- hand written, no marker\n\t- nested " + marker + "  \nMonday\n"
	day := time.Date(2026, 11, 8, 21, 0, 0, 0, mustLocation(t, "America/Chicago"))
	got, ok, err := FindMarkedLine(note, "Hackastak", "My Repo", day)
	if err != nil || !ok {
		t.Fatalf("find = %#v, %v, %v", got, ok, err)
	}
	if !strings.HasPrefix(got.Text, "\t- nested ") || !strings.Contains(got.Text, marker) {
		t.Fatalf("line = %q", got.Text)
	}
	lines := strings.Split(note, "\n")
	if lines[got.Index] != got.Text {
		t.Fatalf("index drifted: %q", lines[got.Index])
	}
}
