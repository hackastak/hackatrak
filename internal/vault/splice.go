package vault

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// MissingHeading is what Splice does when a bucket, daily journal, or weekday
// heading is not in the note.
type MissingHeading int

const (
	// SkipMissing leaves the note unchanged, logs a clear reason, and returns
	// the locate error. It is the zero value.
	SkipMissing MissingHeading = iota
	// CreateMissing inserts the missing heading (and the bullet), and logs
	// what it created. A present heading is never duplicated.
	CreateMissing
)

// Bullet is one auto line for a project-day.
// Text is a single visible line. Splice appends `<!-- ht:<slug> -->` when
// Text does not already end with that marker.
type Bullet struct {
	Slug string
	Text string
}

// SpliceOptions controls missing-heading handling and where that is logged.
// A nil Log writes to stderr. Success does not log.
type SpliceOptions struct {
	Missing MissingHeading
	Log     *log.Logger
}

// Splice inserts or replaces the marked bullet for bullet.Slug under bucket's
// weekday for day. A line that already carries that marker is replaced in
// place; otherwise the bullet is inserted after the day's existing content.
// Lines without that marker are not modified. A second call with the same
// bullet returns the same note.
func Splice(note, bucket string, day time.Time, bullet Bullet, opt SpliceOptions) (string, error) {
	line, err := bulletLine(bullet)
	if err != nil {
		return note, err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return note, fmt.Errorf("vault: bucket is empty")
	}
	weekday := day.Weekday().String()
	section, err := LocateSection(note, bucket, day)
	if err == nil {
		lines := replaceOrInsert(splitLines(note), section, bullet.Slug, line)
		return joinNote(lines, strings.HasSuffix(note, "\n")), nil
	}
	if !isMissingHeading(err) {
		return note, err
	}
	logger := spliceLogger(opt.Log)
	if opt.Missing != CreateMissing {
		logSkip(logger, bullet.Slug, bucket, weekday, err)
		return note, err
	}
	updated, created, err := createMissing(note, bucket, weekday, line, err)
	if err != nil {
		logSkip(logger, bullet.Slug, bucket, weekday, err)
		return note, err
	}
	logCreated(logger, bucket, weekday, created)
	return updated, nil
}

func bulletLine(b Bullet) (string, error) {
	if err := validateSlug(b.Slug); err != nil {
		return "", err
	}
	text := strings.TrimRight(b.Text, " \t")
	if text == "" || strings.ContainsAny(text, "\n\r") {
		return "", fmt.Errorf("vault: bullet text must be a single non-empty line")
	}
	marker, err := Marker(b.Slug)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(text, marker) {
		if !onlySlug(text, b.Slug) {
			return "", fmt.Errorf("%w: bullet marker does not match slug", ErrInvalidSlug)
		}
		return text, nil
	}
	if markerRe.MatchString(text) {
		return "", fmt.Errorf("%w: bullet already has a marker", ErrInvalidSlug)
	}
	return text + " " + marker, nil
}

func onlySlug(line, slug string) bool {
	slugs := markerSlugs(line)
	return len(slugs) == 1 && slugs[0] == slug
}

func replaceOrInsert(lines []string, section Section, slug, line string) []string {
	end := section.End
	if end > len(lines) {
		end = len(lines)
	}
	start := section.Start
	if start < 0 {
		start = 0
	}
	for i := start; i < end; i++ {
		if lineHasSlug(lines[i], slug) {
			out := append([]string(nil), lines...)
			out[i] = line
			return out
		}
	}
	return insertLines(lines, insertionIndex(lines, start, end), []string{line})
}

func insertionIndex(lines []string, start, end int) int {
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		start = end
	}
	at := start
	for i := start; i < end; i++ {
		if strings.TrimSpace(strings.TrimRight(lines[i], "\r")) != "" {
			at = i + 1
		}
	}
	return at
}

func insertLines(lines []string, at int, block []string) []string {
	if at < 0 {
		at = 0
	}
	if at > len(lines) {
		at = len(lines)
	}
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:at]...)
	out = append(out, block...)
	out = append(out, lines[at:]...)
	return out
}

func joinNote(lines []string, trailingNL bool) string {
	if len(lines) == 0 {
		if trailingNL {
			return "\n"
		}
		return ""
	}
	note := strings.Join(lines, "\n")
	if trailingNL && !strings.HasSuffix(note, "\n") {
		return note + "\n"
	}
	return note
}

func isMissingHeading(err error) bool {
	return errors.Is(err, ErrBucketNotFound) ||
		errors.Is(err, ErrJournalNotFound) ||
		errors.Is(err, ErrWeekdayNotFound)
}

func createMissing(note, bucket, weekday, line string, cause error) (string, []string, error) {
	lines := splitLines(note)
	var (
		updated []string
		created []string
		err     error
	)
	switch {
	case errors.Is(cause, ErrBucketNotFound):
		updated = appendBucket(lines, bucket, weekday, line)
		created = []string{"bucket", "journal", "weekday"}
	case errors.Is(cause, ErrJournalNotFound):
		updated, err = insertJournal(lines, bucket, weekday, line)
		created = []string{"journal", "weekday"}
	case errors.Is(cause, ErrWeekdayNotFound):
		updated, err = insertWeekday(lines, bucket, weekday, line)
		created = []string{"weekday"}
	default:
		return note, nil, cause
	}
	if err != nil {
		return note, nil, err
	}
	return joinNote(updated, true), created, nil
}

func appendBucket(lines []string, bucket, weekday, bullet string) []string {
	block := []string{"# " + bucket, "", "#### " + dailyJournalHeading, weekday, bullet}
	return insertSeparated(lines, len(lines), block)
}

func insertJournal(lines []string, bucket, weekday, bullet string) ([]string, error) {
	bucketAt, ok := findBucket(lines, bucket)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrBucketNotFound, bucket)
	}
	at := nextHeading(lines, bucketAt+1, 1)
	block := []string{"#### " + dailyJournalHeading, weekday, bullet}
	return insertSeparated(lines, at, block), nil
}

func insertWeekday(lines []string, bucket, weekday, bullet string) ([]string, error) {
	bucketAt, ok := findBucket(lines, bucket)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrBucketNotFound, bucket)
	}
	journalAt, ok := findJournal(lines, bucketAt+1, nextHeading(lines, bucketAt+1, 1))
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrJournalNotFound, bucket)
	}
	journalEnd := nextHeading(lines, journalAt+1, 0)
	target := weekdayIndex(weekday)
	at := journalEnd
	for i := journalAt + 1; i < journalEnd && i < len(lines); i++ {
		if !isAnyWeekdayLine(lines[i]) {
			continue
		}
		if weekdayIndex(strings.TrimRight(lines[i], " \t\r")) > target {
			at = i
			break
		}
	}
	block := []string{weekday, bullet}
	if at > 0 && strings.TrimRight(lines[at-1], " \t\r") != "" && !isJournalLine(lines[at-1]) {
		block = append([]string{""}, block...)
	}
	if at < len(lines) && strings.TrimRight(lines[at], " \t\r") != "" {
		block = append(block, "")
	}
	return insertLines(lines, at, block), nil
}

func insertSeparated(lines []string, at int, block []string) []string {
	if at > len(lines) {
		at = len(lines)
	}
	if at > 0 && lines[at-1] == "" {
		at--
	}
	if at > 0 && strings.TrimRight(lines[at-1], " \t\r") != "" {
		block = append([]string{""}, block...)
	}
	if at < len(lines) && strings.TrimRight(lines[at], " \t\r") != "" {
		block = append(block, "")
	}
	return insertLines(lines, at, block)
}

func isJournalLine(line string) bool {
	level, text, ok := atxHeading(line)
	return ok && level == 4 && text == dailyJournalHeading
}

func weekdayIndex(name string) int {
	switch name {
	case "Monday":
		return 0
	case "Tuesday":
		return 1
	case "Wednesday":
		return 2
	case "Thursday":
		return 3
	case "Friday":
		return 4
	case "Saturday":
		return 5
	case "Sunday":
		return 6
	default:
		return -1
	}
}

func spliceLogger(l *log.Logger) *log.Logger {
	if l != nil {
		return l
	}
	return log.New(os.Stderr, "", 0)
}

func logSkip(l *log.Logger, slug, bucket, weekday string, err error) {
	l.Printf("skipping splice for %q on %s under %q: %s", slug, weekday, bucket, err.Error())
}

func logCreated(l *log.Logger, bucket, weekday string, created []string) {
	for _, kind := range created {
		switch kind {
		case "bucket":
			l.Printf("created bucket heading %q", bucket)
		case "journal":
			l.Printf("created daily journal under %q", bucket)
		case "weekday":
			l.Printf("created weekday heading %q under %q", weekday, bucket)
		}
	}
}
