package vault

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrInvalidSlug means the slug cannot be embedded in an HTML comment marker.
var ErrInvalidSlug = errors.New("vault: invalid marker slug")

// markerRe is the invisible per-project marker. The slug may contain spaces;
// the comment delimiters stay exactly `<!-- ht:` and ` -->`.
var markerRe = regexp.MustCompile(`<!-- ht:(.*?) -->`)

// MarkedLine is one bullet under today's weekday that already carries a marker.
// Index matches strings.Split(note, "\n").
type MarkedLine struct {
	Index int
	Text  string
	Slug  string
}

// Marker returns the invisible comment for slug: `<!-- ht:<slug> -->`.
func Marker(slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}
	return "<!-- ht:" + slug + " -->", nil
}

// MarkerSlug reports the first valid Hackatrak marker on line.
func MarkerSlug(line string) (string, bool) {
	slugs := markerSlugs(line)
	if len(slugs) == 0 {
		return "", false
	}
	return slugs[0], true
}

// FindMarkedLine returns the first line under today's weekday in bucket whose
// marker slug matches. A missing section is an error. A present section with
// no matching marker returns ok=false.
func FindMarkedLine(note, bucket, slug string, day time.Time) (MarkedLine, bool, error) {
	if err := validateSlug(slug); err != nil {
		return MarkedLine{}, false, err
	}
	section, err := LocateSection(note, bucket, day)
	if err != nil {
		return MarkedLine{}, false, err
	}
	lines := splitLines(note)
	end := section.End
	if end > len(lines) {
		end = len(lines)
	}
	for i := section.Start; i < end; i++ {
		if lineHasSlug(lines[i], slug) {
			return MarkedLine{Index: i, Text: lines[i], Slug: slug}, true, nil
		}
	}
	return MarkedLine{}, false, nil
}

func lineHasSlug(line, slug string) bool {
	for _, got := range markerSlugs(line) {
		if got == slug {
			return true
		}
	}
	return false
}

func markerSlugs(line string) []string {
	matches := markerRe.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}
	slugs := make([]string, 0, len(matches))
	for _, match := range matches {
		slug := match[1]
		if validateSlug(slug) != nil {
			continue
		}
		slugs = append(slugs, slug)
	}
	return slugs
}

func validateSlug(slug string) error {
	if slug == "" || strings.TrimSpace(slug) != slug {
		return fmt.Errorf("%w: empty or padded", ErrInvalidSlug)
	}
	if strings.ContainsAny(slug, "<>\n\r") || strings.Contains(slug, "--") {
		return fmt.Errorf("%w: %q", ErrInvalidSlug, slug)
	}
	return nil
}
