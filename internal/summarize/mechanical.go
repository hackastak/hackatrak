package summarize

import (
	"fmt"
	"regexp"
	"strings"
)

// maxSummaryRunes is the budget for a joined line before later subjects are
// counted instead of appended. One subject is never cut mid-phrase.
const maxSummaryRunes = 160

// conventionalPrefix matches a conventional-commit type prefix. The remainder
// has to be non-empty or the prefix stays — "fix:" alone is still a subject.
var conventionalPrefix = regexp.MustCompile(`^(?i)(?:feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert)(?:\([^)]+\))?!?:\s+`)

// MechanicalSummarizer condenses commit subjects into one readable line.
// The zero value is ready to use. It does not extract tickets or invent
// file-touch text; those are separate steps behind the same Summarizer seam.
type MechanicalSummarizer struct{}

// Summarize joins cleaned commit subjects in caller order.
//
// A subject is trimmed, internal whitespace is collapsed, and one trailing
// period is removed so the clause can sit after a colon. Conventional type
// prefixes are stripped. Merge commits and empty subjects are dropped.
// A repeated subject, compared case-insensitively, keeps the first wording.
// Subjects that would push the joined line past maxSummaryRunes are omitted
// and counted as "(+N more)". The input day is not modified.
func (MechanicalSummarizer) Summarize(day Day) (Summary, error) {
	subjects := condenseSubjects(day.Commits)
	return Summary{Line: subjects}, nil
}

func condenseSubjects(commits []Commit) string {
	kept := make([]string, 0, len(commits))
	seen := make(map[string]struct{}, len(commits))
	for _, commit := range commits {
		subject, ok := cleanSubject(commit.Subject)
		if !ok {
			continue
		}
		key := strings.ToLower(subject)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		kept = append(kept, subject)
	}
	if len(kept) == 0 {
		return ""
	}

	line := kept[0]
	used := 1
	for _, subject := range kept[1:] {
		next := line + "; " + subject
		if len([]rune(next)) > maxSummaryRunes {
			break
		}
		line = next
		used++
	}
	if omitted := len(kept) - used; omitted > 0 {
		line += fmt.Sprintf(" (+%d more)", omitted)
	}
	return line
}

func cleanSubject(raw string) (string, bool) {
	subject := strings.Join(strings.Fields(raw), " ")
	subject = strings.TrimSuffix(subject, ".")
	subject = strings.TrimSpace(subject)
	if subject == "" || isMergeSubject(subject) {
		return "", false
	}
	if rest := conventionalPrefix.ReplaceAllString(subject, ""); rest != "" && rest != subject {
		subject = strings.TrimSpace(rest)
	}
	if subject == "" || isMergeSubject(subject) {
		return "", false
	}
	return subject, true
}

func isMergeSubject(subject string) bool {
	folded := strings.ToLower(subject)
	return strings.HasPrefix(folded, "merge branch ") ||
		strings.HasPrefix(folded, "merge pull request ") ||
		strings.HasPrefix(folded, "merge remote-tracking branch ")
}
