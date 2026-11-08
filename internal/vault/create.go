package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	templateRel    = "_templates/Weekly_note.md"
	dateLineMarker = "Notes for the week of"
)

// ErrNoDateLine is returned when the weekly template has no date line to fill.
var ErrNoDateLine = errors.New("vault: weekly template has no date line")

var templaterTag = regexp.MustCompile(`<%.*?%>`)

// EnsureWeekNote returns the weekly note path for day, creating it from
// _templates/Weekly_note.md when it is missing. An existing note is left
// unchanged. The date line is filled with that week's Monday (YYYY-MM-DD).
// Raw Templater tags and the word "undefined" are never written.
func EnsureWeekNote(vaultPath string, day time.Time) (string, error) {
	path, err := NotePath(vaultPath, day)
	if err != nil {
		return "", err
	}
	_, err = os.Stat(path)
	if err == nil {
		return path, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("vault: stat week note: %w", err)
	}

	body, err := renderWeekTemplate(vaultPath, day)
	if err != nil {
		return "", err
	}
	if err := writeNewFile(path, []byte(body)); err != nil {
		if errors.Is(err, os.ErrExist) {
			return path, nil
		}
		return "", err
	}
	return path, nil
}

func renderWeekTemplate(vaultPath string, day time.Time) (string, error) {
	raw, err := os.ReadFile(filepath.Join(vaultPath, filepath.FromSlash(templateRel)))
	if err != nil {
		return "", fmt.Errorf("vault: read weekly template: %w", err)
	}
	return fillDateLine(string(raw), WeekMonday(day))
}

func fillDateLine(template string, monday time.Time) (string, error) {
	date := monday.Format("2006-01-02")
	lines := strings.Split(template, "\n")
	filled := make([]string, len(lines))
	found := false
	for i, line := range lines {
		if strings.Contains(line, dateLineMarker) {
			found = true
			line = templaterTag.ReplaceAllString(line, date)
			line = strings.ReplaceAll(line, "undefined", date)
			if strings.Contains(line, "<%") || strings.Contains(line, "undefined") {
				return "", fmt.Errorf("vault: date line still has templater output")
			}
		}
		filled[i] = line
	}
	if !found {
		return "", ErrNoDateLine
	}
	return strings.Join(filled, "\n"), nil
}

func writeNewFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("vault: create week dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("vault: create week note: %w", err)
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(path)
		return fmt.Errorf("vault: write week note: %w", werr)
	}
	if cerr != nil {
		_ = os.Remove(path)
		return fmt.Errorf("vault: close week note: %w", cerr)
	}
	return nil
}
