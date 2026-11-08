package summarize

import (
	"fmt"
	"regexp"
	"strings"
)

// extractTicket returns the first ticket_regex match.
// Branches are tried before commit messages: the day branch, then each
// commit branch, then subjects, then bodies. Within a kind, caller order wins.
// An empty pattern matches nothing. An invalid pattern is an error.
// The returned id is the full match, with no parentheses.
func extractTicket(pattern string, day Day) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("summarize: ticket_regex: %w", err)
	}
	for _, text := range ticketSources(day) {
		if ticket := re.FindString(text); ticket != "" {
			return ticket, nil
		}
	}
	return "", nil
}

func ticketSources(day Day) []string {
	sources := make([]string, 0, 1+len(day.Commits)*3)
	sources = append(sources, day.Branch)
	for _, commit := range day.Commits {
		sources = append(sources, commit.Branch)
	}
	for _, commit := range day.Commits {
		sources = append(sources, commit.Subject)
	}
	for _, commit := range day.Commits {
		sources = append(sources, commit.Body)
	}
	return sources
}
