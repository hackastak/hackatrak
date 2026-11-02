package config

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	indexedField = regexp.MustCompile(`^([A-Za-z0-9_]+)\[(\d+)\]\.([A-Za-z0-9_]+)$`)
	assignLine   = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=`)
	arrayTable   = regexp.MustCompile(`^\s*\[\[([A-Za-z0-9_-]+)\]\]\s*(?:#.*)?$`)
	stdTable     = regexp.MustCompile(`^\s*\[([A-Za-z0-9_-]+)\]\s*(?:#.*)?$`)
)

// lineForField returns the 1-based source line of a config field, or 0 if the
// key is absent. Field forms match validation: "role", "git.author_emails",
// "roots[0].path".
func lineForField(src, field string) int {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	if m := indexedField.FindStringSubmatch(field); m != nil {
		idx, err := strconv.Atoi(m[2])
		if err != nil {
			return 0
		}
		return lineInArrayTable(lines, m[1], idx, m[3])
	}
	if table, key, ok := strings.Cut(field, "."); ok {
		return lineInTable(lines, table, key)
	}
	if n := lineInTable(lines, "", field); n > 0 {
		return n
	}
	return lineAnywhere(lines, field)
}

func lineInArrayTable(lines []string, name string, idx int, key string) int {
	seen := -1
	active := false
	for i, line := range lines {
		if m := arrayTable.FindStringSubmatch(line); m != nil {
			if m[1] == name {
				seen++
				active = seen == idx
			} else {
				active = false
			}
			continue
		}
		if stdTable.MatchString(line) {
			active = false
			continue
		}
		if active && assigned(line, key) {
			return i + 1
		}
	}
	return 0
}

func lineInTable(lines []string, table, key string) int {
	current := ""
	for i, line := range lines {
		if m := arrayTable.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if m := stdTable.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if current == table && assigned(line, key) {
			return i + 1
		}
	}
	return 0
}

func lineAnywhere(lines []string, key string) int {
	for i, line := range lines {
		if assigned(line, key) {
			return i + 1
		}
	}
	return 0
}

func assigned(line, key string) bool {
	m := assignLine.FindStringSubmatch(line)
	return m != nil && m[1] == key
}
