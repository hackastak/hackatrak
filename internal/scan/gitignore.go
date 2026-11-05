package scan

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoreRule is one pattern from a .gitignore. base is the directory that
// contains that file. Rules are applied parent-first; the last match wins.
type ignoreRule struct {
	base     string
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool
}

// ignoreState is the .gitignore stack for one walk root.
// Ancestor rules inside the enclosing repo are loaded up front. Rules from
// directories entered during the walk are pushed and popped with the walk.
type ignoreState struct {
	rules  []ignoreRule
	levels []ignoreLevel
}

type ignoreLevel struct {
	dir string
	n   int
}

func newIgnoreState(walkRoot string) (*ignoreState, error) {
	rules, err := ancestorGitignores(walkRoot)
	if err != nil {
		return nil, err
	}
	own, err := readGitignore(walkRoot)
	if err != nil {
		return nil, err
	}
	rules = append(rules, own...)
	return &ignoreState{
		rules:  rules,
		levels: []ignoreLevel{{dir: walkRoot, n: len(rules)}},
	}, nil
}

// trim drops rules from directories that are not path or an ancestor of it.
func (s *ignoreState) trim(path string) {
	for len(s.levels) > 1 {
		top := s.levels[len(s.levels)-1].dir
		if top == path || isAncestorDir(top, path) {
			return
		}
		s.levels = s.levels[:len(s.levels)-1]
		s.rules = s.rules[:s.levels[len(s.levels)-1].n]
	}
}

// enter adds dir's .gitignore. dir must already have survived the ignore check.
func (s *ignoreState) enter(dir string) error {
	rules, err := readGitignore(dir)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	s.rules = append(s.rules, rules...)
	s.levels = append(s.levels, ignoreLevel{dir: dir, n: len(s.rules)})
	return nil
}

// ignored reports whether git would ignore path given the rules loaded so far.
// A file under an ignored directory is ignored, and a later negation cannot
// bring it back. That matches git: a parent exclude blocks re-inclusion.
func (s *ignoreState) ignored(abs string, isDir bool) bool {
	if s.matched(abs, isDir) {
		return true
	}
	if isDir {
		return false
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if s.matched(dir, true) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
	}
}

func (s *ignoreState) matched(abs string, isDir bool) bool {
	ignored := false
	for _, rule := range s.rules {
		rel, err := filepath.Rel(rule.base, abs)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		if rule.matches(filepath.ToSlash(rel), isDir) {
			ignored = !rule.negate
		}
	}
	return ignored
}

func (r ignoreRule) matches(rel string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	if !r.anchored {
		if r.pattern == "**" {
			return true
		}
		return matchSegment(r.pattern, path.Base(rel))
	}
	return matchPath(r.pattern, rel)
}

func ancestorGitignores(walkRoot string) ([]ignoreRule, error) {
	repo, ok, err := enclosingRepo(walkRoot)
	if err != nil || !ok || repo == walkRoot {
		return nil, err
	}
	var dirs []string
	for cur := filepath.Dir(walkRoot); ; cur = filepath.Dir(cur) {
		if cur != repo && !isAncestorDir(repo, cur) {
			break
		}
		dirs = append(dirs, cur)
		if cur == repo {
			break
		}
	}
	var rules []ignoreRule
	for i := len(dirs) - 1; i >= 0; i-- {
		more, err := readGitignore(dirs[i])
		if err != nil {
			return nil, err
		}
		rules = append(rules, more...)
	}
	return rules, nil
}

func enclosingRepo(dir string) (string, bool, error) {
	cur := dir
	for {
		ok, err := hasGitMetadata(cur)
		if err != nil {
			return "", false, err
		}
		if ok {
			return cur, true, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false, nil
		}
		cur = parent
	}
}

func readGitignore(dir string) ([]ignoreRule, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseGitignore(dir, string(raw)), nil
}

func parseGitignore(base, text string) []ignoreRule {
	var rules []ignoreRule
	for _, line := range strings.Split(text, "\n") {
		rule, ok := parseIgnoreLine(base, line)
		if ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseIgnoreLine(base, line string) (ignoreRule, bool) {
	line = strings.TrimSuffix(line, "\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	line = trimUnescapedTrailingSpace(line)
	if line == "" {
		return ignoreRule{}, false
	}
	negate := false
	switch {
	case strings.HasPrefix(line, `\!`), strings.HasPrefix(line, `\#`):
		line = line[1:]
	case strings.HasPrefix(line, "!"):
		negate = true
		line = line[1:]
		if line == "" {
			return ignoreRule{}, false
		}
	}
	dirOnly := strings.HasSuffix(line, "/")
	if dirOnly {
		line = strings.TrimSuffix(line, "/")
	}
	anchored := strings.HasPrefix(line, "/") || strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return ignoreRule{}, false
	}
	return ignoreRule{
		base:     base,
		pattern:  line,
		negate:   negate,
		dirOnly:  dirOnly,
		anchored: anchored,
	}, true
}

func trimUnescapedTrailingSpace(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		if len(s) > 1 && s[len(s)-2] == '\\' {
			return s[:len(s)-2] + " "
		}
		s = s[:len(s)-1]
	}
	return s
}

func isAncestorDir(dir, child string) bool {
	rel, err := filepath.Rel(dir, child)
	if err != nil || rel == "." {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func matchPath(pattern, rel string) bool {
	return matchSegs(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return len(segs) > 0
			}
			for i := 0; i <= len(segs); i++ {
				if matchSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 || !matchSegment(pat[0], segs[0]) {
			return false
		}
		pat = pat[1:]
		segs = segs[1:]
	}
	return len(segs) == 0
}

func matchSegment(pattern, name string) bool {
	return matchRunes([]rune(pattern), []rune(name))
}

func matchRunes(pat, name []rune) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			pat = pat[1:]
			for len(pat) > 0 && pat[0] == '*' {
				pat = pat[1:]
			}
			if len(pat) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if matchRunes(pat, name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(name) == 0 {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		case '[':
			class, rest, ok := parseClass(pat)
			if !ok {
				if len(name) == 0 || name[0] != '[' {
					return false
				}
				pat = pat[1:]
				name = name[1:]
				continue
			}
			if len(name) == 0 || !class.contains(name[0]) {
				return false
			}
			pat = rest
			name = name[1:]
		case '\\':
			if len(pat) == 1 {
				return len(name) == 1 && name[0] == '\\'
			}
			pat = pat[1:]
			if len(name) == 0 || name[0] != pat[0] {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		default:
			if len(name) == 0 || name[0] != pat[0] {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		}
	}
	return len(name) == 0
}

type runeClass struct {
	negate bool
	runes  []rune
	ranges [][2]rune
}

func (c runeClass) contains(r rune) bool {
	hit := false
	for _, one := range c.runes {
		if one == r {
			hit = true
			break
		}
	}
	if !hit {
		for _, span := range c.ranges {
			if r >= span[0] && r <= span[1] {
				hit = true
				break
			}
		}
	}
	if c.negate {
		return !hit
	}
	return hit
}

func parseClass(pat []rune) (runeClass, []rune, bool) {
	if len(pat) < 2 || pat[0] != '[' {
		return runeClass{}, nil, false
	}
	i := 1
	var class runeClass
	if i < len(pat) && (pat[i] == '!' || pat[i] == '^') {
		class.negate = true
		i++
	}
	closed := false
	for i < len(pat) {
		if pat[i] == ']' && i > 1 {
			closed = true
			i++
			break
		}
		start := pat[i]
		if start == '\\' && i+1 < len(pat) {
			start = pat[i+1]
			i += 2
		} else {
			i++
		}
		if i+1 < len(pat) && pat[i] == '-' && pat[i+1] != ']' {
			end := pat[i+1]
			i += 2
			if end == '\\' && i < len(pat) {
				end = pat[i]
				i++
			}
			if end < start {
				start, end = end, start
			}
			class.ranges = append(class.ranges, [2]rune{start, end})
			continue
		}
		class.runes = append(class.runes, start)
	}
	if !closed || (len(class.runes) == 0 && len(class.ranges) == 0) {
		return runeClass{}, nil, false
	}
	return class, pat[i:], true
}
