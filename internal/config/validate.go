package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FieldError is one config field that failed validation or parsing.
// Line is 1-based and 0 when the source location is unknown.
type FieldError struct {
	Field string
	Line  int
	Msg   string
}

func (e *FieldError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s at line %d: %s", e.Field, e.Line, e.Msg)
	}
	return e.Field + ": " + e.Msg
}

// accountRe is the filename segment used in spool-<account>-<YYYY>-W<ww>.json.
var accountRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Validate checks the rules that a loaded config must satisfy.
// Paths are checked as given, so call this after ~ expansion.
// spool_dir is not required to exist; the tool creates it later.
// An empty spool_dir is the default vault-relative directory.
// Each root names a life bucket. An account may have more than one.
func Validate(cfg Config) error {
	var errs []error
	errs = append(errs, validatePath("vault_path", cfg.VaultPath)...)
	if err := CheckAccount(cfg.Account); err != nil {
		errs = append(errs, err)
	}
	if len(cfg.Roots) == 0 {
		errs = append(errs, &FieldError{Field: "roots", Msg: "must contain at least one entry"})
	}
	for i, root := range cfg.Roots {
		errs = append(errs, validatePath(fmt.Sprintf("roots[%d].path", i), root.Path)...)
		if strings.TrimSpace(root.Bucket) == "" {
			errs = append(errs, &FieldError{
				Field: fmt.Sprintf("roots[%d].bucket", i),
				Msg:   "required",
			})
		}
	}
	if cfg.Role != RoleWriter && cfg.Role != RoleSpooler {
		errs = append(errs, &FieldError{Field: "role", Msg: `must be "writer" or "spooler"`})
	}
	if _, err := ResolveSpoolDir(cfg); err != nil {
		errs = append(errs, err)
	}
	if cfg.Summarizer != "" && cfg.Summarizer != SummarizerMechanical && cfg.Summarizer != SummarizerOllama {
		errs = append(errs, &FieldError{Field: "summarizer", Msg: `must be "mechanical" or "ollama"`})
	}
	return errors.Join(errs...)
}

// CheckAccount reports whether account can appear in a spool filename.
// It must be one segment: letters, digits, ".", "_", or "-", and must not
// contain "..".
func CheckAccount(account string) error {
	if strings.TrimSpace(account) == "" {
		return &FieldError{Field: "account", Msg: "required"}
	}
	if account != strings.TrimSpace(account) || strings.Contains(account, "..") || !accountRe.MatchString(account) {
		return &FieldError{Field: "account", Msg: `must be one filename segment of letters, digits, ".", "_", or "-"`}
	}
	return nil
}

// Buckets returns each roots[].bucket, trimmed, in config order.
// An empty bucket is an error. Duplicate headings are kept.
func Buckets(cfg Config) ([]string, error) {
	if len(cfg.Roots) == 0 {
		return nil, &FieldError{Field: "roots", Msg: "must contain at least one entry"}
	}
	buckets := make([]string, len(cfg.Roots))
	for i, root := range cfg.Roots {
		bucket := strings.TrimSpace(root.Bucket)
		if bucket == "" {
			return nil, &FieldError{Field: fmt.Sprintf("roots[%d].bucket", i), Msg: "required"}
		}
		buckets[i] = bucket
	}
	return buckets, nil
}

// ResolveSpoolDir returns the absolute spool directory inside the vault.
// An empty SpoolDir uses DefaultSpoolDir. A relative path is joined to
// VaultPath. The directory is not created and does not need to exist.
func ResolveSpoolDir(cfg Config) (string, error) {
	dir := strings.TrimSpace(cfg.SpoolDir)
	if dir == "" {
		dir = DefaultSpoolDir
	}
	if strings.Contains(dir, "..") {
		return "", &FieldError{Field: "spool_dir", Msg: "must not contain .."}
	}
	vault := strings.TrimSpace(cfg.VaultPath)
	if vault == "" {
		return "", &FieldError{Field: "vault_path", Msg: "required"}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(vault, filepath.FromSlash(dir))
	}
	vault = filepath.Clean(vault)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(vault, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &FieldError{Field: "spool_dir", Msg: "must be inside the vault"}
	}
	return dir, nil
}

func validatePath(field, path string) []error {
	if path == "" {
		return []error{&FieldError{Field: field, Msg: "required"}}
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []error{&FieldError{Field: field, Msg: "path does not exist: " + path}}
		}
		return []error{&FieldError{Field: field, Msg: err.Error()}}
	}
	if !info.IsDir() {
		return []error{&FieldError{Field: field, Msg: "not a directory: " + path}}
	}
	return nil
}
