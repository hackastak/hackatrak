package scan

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	commitFieldSep  = "\x1f"
	commitRecordSep = "\x1e"
	// commitLogFormat is hash, author time, source ref, subject, body.
	commitLogFormat = "%H%x1f%aI%x1f%S%x1f%s%x1f%b%x1e"
)

// Commit is one commit from git log.
// Branch is the local branch git walked to reach the commit.
// Time keeps the author-date offset from git.
type Commit struct {
	Hash    string
	Time    time.Time
	Branch  string
	Subject string
	Body    string
}

// LocalMidnight returns the start of now's calendar day in now's location.
func LocalMidnight(now time.Time) time.Time {
	local := now.In(now.Location())
	year, month, day := local.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, local.Location())
}

// CommitsSince lists commits reachable from local branches.
//
// It runs `git log --since=<since> --author=<email> --pretty=<format>`.
// since is the sync cursor or an explicit local-midnight timestamp.
// An empty since uses local midnight. Git's --since includes a commit
// whose author time equals since, at second resolution.
// authors are email addresses; each is matched literally. At least one
// is required so a shared repo cannot log someone else's commits.
// Results are oldest first.
func CommitsSince(repo, since string, authors []string) ([]Commit, error) {
	if strings.TrimSpace(repo) == "" {
		return nil, fmt.Errorf("commits since: repo path is required")
	}
	emails, err := authorEmails(authors)
	if err != nil {
		return nil, fmt.Errorf("commits since: %w", err)
	}
	if strings.TrimSpace(since) == "" {
		since = LocalMidnight(time.Now()).Format(time.RFC3339)
	}

	args := []string{
		"log",
		"--reverse",
		"--branches",
		"--source",
		"--since", since,
		"--pretty=format:" + commitLogFormat,
	}
	for _, email := range emails {
		args = append(args, "--author="+email)
	}

	out, err := runGit(repo, args...)
	if err != nil {
		return nil, fmt.Errorf("commits since %s: %w", repo, err)
	}
	commits, err := parseCommitLog(out)
	if err != nil {
		return nil, fmt.Errorf("commits since %s: %w", repo, err)
	}
	return commits, nil
}

func authorEmails(authors []string) ([]string, error) {
	emails := make([]string, 0, len(authors))
	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author == "" {
			continue
		}
		emails = append(emails, regexp.QuoteMeta(author))
	}
	if len(emails) == 0 {
		return nil, fmt.Errorf("at least one author email is required")
	}
	return emails, nil
}

func parseCommitLog(output string) ([]Commit, error) {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return nil, nil
	}

	records := strings.Split(output, commitRecordSep)
	commits := make([]Commit, 0, len(records))
	for _, record := range records {
		record = strings.TrimPrefix(record, "\n")
		if record == "" {
			continue
		}
		commit, err := parseCommitRecord(record)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func parseCommitRecord(record string) (Commit, error) {
	fields := strings.Split(record, commitFieldSep)
	if len(fields) != 5 {
		return Commit{}, fmt.Errorf("parse commit log: want 5 fields, got %d", len(fields))
	}
	if fields[0] == "" {
		return Commit{}, fmt.Errorf("parse commit log: empty hash")
	}
	when, err := time.Parse(time.RFC3339, fields[1])
	if err != nil {
		return Commit{}, fmt.Errorf("parse commit time %q: %w", fields[1], err)
	}
	return Commit{
		Hash:    fields[0],
		Time:    when,
		Branch:  branchName(fields[2]),
		Subject: fields[3],
		Body:    strings.TrimRight(fields[4], "\n"),
	}, nil
}

func branchName(ref string) string {
	ref = strings.TrimSpace(ref)
	return strings.TrimPrefix(ref, "refs/heads/")
}
