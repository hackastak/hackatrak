package vault

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const dailyJournalHeading = "Daily Journal"

// ErrBucketNotFound means the note has no `# <bucket>` heading.
var ErrBucketNotFound = errors.New("vault: bucket heading not found")

// ErrJournalNotFound means the bucket has no `#### Daily Journal` heading.
var ErrJournalNotFound = errors.New("vault: daily journal not found")

// ErrWeekdayNotFound means today's weekday heading is missing under that journal.
var ErrWeekdayNotFound = errors.New("vault: weekday heading not found")

// Section is one weekday under a bucket's Daily Journal.
// Heading is the weekday line. Start is the first line after it.
// End is exclusive: the next weekday line, the next ATX heading, or EOF.
// Indexes match strings.Split(note, "\n").
type Section struct {
	Bucket  string
	Weekday string
	Heading int
	Start   int
	End     int
}

// LocateSection finds bucket heading → #### Daily Journal → today's weekday.
// day is already in the local zone; its Weekday is used as-is.
func LocateSection(note, bucket string, day time.Time) (Section, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return Section{}, fmt.Errorf("vault: bucket is empty")
	}
	weekday := day.Weekday().String()
	lines := splitLines(note)

	bucketAt, ok := findBucket(lines, bucket)
	if !ok {
		return Section{}, fmt.Errorf("%w: %s", ErrBucketNotFound, bucket)
	}
	journalAt, ok := findJournal(lines, bucketAt+1, nextHeading(lines, bucketAt+1, 1))
	if !ok {
		return Section{}, fmt.Errorf("%w: %s", ErrJournalNotFound, bucket)
	}
	journalEnd := nextHeading(lines, journalAt+1, 0)
	dayAt, ok := findWeekday(lines, journalAt+1, journalEnd, weekday)
	if !ok {
		return Section{}, fmt.Errorf("%w: %s %s", ErrWeekdayNotFound, bucket, weekday)
	}
	return Section{
		Bucket:  bucket,
		Weekday: weekday,
		Heading: dayAt,
		Start:   dayAt + 1,
		End:     weekdayEnd(lines, dayAt+1, journalEnd),
	}, nil
}

func splitLines(note string) []string {
	if note == "" {
		return nil
	}
	return strings.Split(note, "\n")
}

func findBucket(lines []string, bucket string) (int, bool) {
	for i, line := range lines {
		level, text, ok := atxHeading(line)
		if ok && level == 1 && text == bucket {
			return i, true
		}
	}
	return 0, false
}

func findJournal(lines []string, start, end int) (int, bool) {
	for i := start; i < end && i < len(lines); i++ {
		level, text, ok := atxHeading(lines[i])
		if ok && level == 4 && text == dailyJournalHeading {
			return i, true
		}
	}
	return 0, false
}

func findWeekday(lines []string, start, end int, weekday string) (int, bool) {
	for i := start; i < end && i < len(lines); i++ {
		if isWeekdayLine(lines[i], weekday) {
			return i, true
		}
	}
	return 0, false
}

func weekdayEnd(lines []string, start, journalEnd int) int {
	for i := start; i < journalEnd && i < len(lines); i++ {
		if isAnyWeekdayLine(lines[i]) {
			return i
		}
	}
	if journalEnd < len(lines) {
		return journalEnd
	}
	return len(lines)
}

// nextHeading returns the index of the next ATX heading at or above minLevel
// (1 is highest). minLevel 0 matches any heading. The index is len(lines)
// when none remains.
func nextHeading(lines []string, start, minLevel int) int {
	for i := start; i < len(lines); i++ {
		level, _, ok := atxHeading(lines[i])
		if !ok {
			continue
		}
		if minLevel == 0 || level <= minLevel {
			return i
		}
	}
	return len(lines)
}

func atxHeading(line string) (level int, text string, ok bool) {
	line = strings.TrimRight(line, " \t\r")
	spaces := 0
	for spaces < len(line) && line[spaces] == ' ' && spaces < 3 {
		spaces++
	}
	line = line[spaces:]
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 || i == len(line) {
		return 0, "", false
	}
	if line[i] != ' ' && line[i] != '\t' {
		return 0, "", false
	}
	return i, strings.TrimSpace(line[i:]), true
}

func isWeekdayLine(line, weekday string) bool {
	return strings.TrimRight(line, " \t\r") == weekday
}

func isAnyWeekdayLine(line string) bool {
	switch strings.TrimRight(line, " \t\r") {
	case "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday":
		return true
	default:
		return false
	}
}
