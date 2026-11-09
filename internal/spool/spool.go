// Package spool defines the cross-account handoff file.
//
// A spooler writes one JSON file per ISO week. A writer reads it later and
// splices the days into the weekly note. This package does not touch the note
// and does not create the file.
package spool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hackastak/hackatrak/internal/config"
)

// Version is the only schema this package reads and writes.
const Version = 1

// Day is one project-day the writer can splice.
// Date is a local calendar day, YYYY-MM-DD. Bucket is the weekly-note heading.
// Text is the visible bullet and must not include the vault marker. Ticket is
// the raw id; empty means the bullet has no parentheses.
type Day struct {
	Date   string `json:"date"`
	Bucket string `json:"bucket"`
	Slug   string `json:"slug"`
	Ticket string `json:"ticket,omitempty"`
	Text   string `json:"text"`
}

// File is one account's ISO week of project-days.
// The on-disk name is spool-<account>-<YYYY>-W<ww>.json. A day carries its
// own bucket so one account can hand off more than one heading.
type File struct {
	Version int    `json:"version"`
	Account string `json:"account"`
	Year    int    `json:"year"`
	Week    int    `json:"week"`
	Days    []Day  `json:"days"`
}

// Name returns spool-<account>-<YYYY>-W<ww>.json.
// Week is the ISO week, 1–53. Account must be safe as a filename segment.
func Name(account string, year, week int) (string, error) {
	if err := config.CheckAccount(account); err != nil {
		return "", fmt.Errorf("spool: %w", err)
	}
	if year < 1 || year > 9999 {
		return "", fmt.Errorf("spool: year %d is out of range", year)
	}
	if week < 1 || week > 53 {
		return "", fmt.Errorf("spool: week %d is out of range", week)
	}
	return fmt.Sprintf("spool-%s-%04d-W%02d.json", account, year, week), nil
}

// Path joins dir and the ISO-week filename for day.
// dir is the spool directory, usually config.DefaultSpoolDir resolved under
// the vault. day uses its own location for the ISO week.
func Path(dir, account string, day time.Time) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("spool: directory is empty")
	}
	year, week := day.ISOWeek()
	name, err := Name(account, year, week)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Parse decodes and checks a spool document.
// days may be empty. A null days array is rejected.
func Parse(data []byte) (File, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return File{}, fmt.Errorf("spool: %w", err)
	}
	if _, ok := raw["days"]; !ok {
		return File{}, fmt.Errorf("spool: days is required")
	}
	if string(bytes.TrimSpace(raw["days"])) == "null" {
		return File{}, fmt.Errorf("spool: days must be an array")
	}

	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file File
	if err := dec.Decode(&file); err != nil {
		return File{}, fmt.Errorf("spool: %w", err)
	}
	if err := file.validate(); err != nil {
		return File{}, err
	}
	if file.Days == nil {
		file.Days = []Day{}
	}
	return file, nil
}

// Bytes encodes file as indented JSON with a trailing newline.
// The result is rejected when file does not satisfy the schema.
func (f File) Bytes() ([]byte, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	if f.Days == nil {
		f.Days = []Day{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, fmt.Errorf("spool: %w", err)
	}
	return buf.Bytes(), nil
}

func (f File) validate() error {
	if f.Version != Version {
		return fmt.Errorf("spool: version %d is not supported", f.Version)
	}
	if err := config.CheckAccount(f.Account); err != nil {
		return fmt.Errorf("spool: %w", err)
	}
	if _, err := Name(f.Account, f.Year, f.Week); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(f.Days))
	for i, day := range f.Days {
		if err := validateDay(f.Year, f.Week, day); err != nil {
			return fmt.Errorf("spool: days[%d]: %w", i, err)
		}
		key := day.Date + "\x00" + day.Slug
		if _, ok := seen[key]; ok {
			return fmt.Errorf("spool: days[%d]: duplicate %s %s", i, day.Date, day.Slug)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateDay(year, week int, day Day) error {
	parsed, err := time.Parse("2006-01-02", day.Date)
	if err != nil {
		return fmt.Errorf("date %q: want YYYY-MM-DD", day.Date)
	}
	gotYear, gotWeek := parsed.ISOWeek()
	if gotYear != year || gotWeek != week {
		return fmt.Errorf("date %s is %04d-W%02d, not %04d-W%02d", day.Date, gotYear, gotWeek, year, week)
	}
	if err := checkSlug(day.Slug); err != nil {
		return err
	}
	if strings.TrimSpace(day.Bucket) == "" {
		return fmt.Errorf("bucket is required")
	}
	text := day.Text
	if strings.TrimSpace(text) == "" || strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("text must be one non-empty line")
	}
	if strings.Contains(text, "<!-- ht:") {
		return fmt.Errorf("text must not include a marker")
	}
	if strings.TrimSpace(day.Ticket) != day.Ticket || strings.ContainsAny(day.Ticket, "\r\n()") {
		return fmt.Errorf("ticket must be a raw id")
	}
	return nil
}

func checkSlug(slug string) error {
	if slug == "" || strings.TrimSpace(slug) != slug {
		return fmt.Errorf("slug is empty or padded")
	}
	if strings.ContainsAny(slug, "<>\n\r") || strings.Contains(slug, "--") {
		return fmt.Errorf("slug %q is invalid", slug)
	}
	return nil
}
