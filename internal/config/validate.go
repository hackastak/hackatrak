package config

import (
	"errors"
	"fmt"
	"os"
)

// FieldError is one config field that failed validation.
type FieldError struct {
	Field string
	Msg   string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Msg
}

// Validate checks the rules that a loaded config must satisfy.
// Paths are checked as given, so call this after ~ expansion.
// spool_dir is not required to exist; the tool creates it later.
func Validate(cfg Config) error {
	var errs []error
	errs = append(errs, validatePath("vault_path", cfg.VaultPath)...)
	if len(cfg.Roots) == 0 {
		errs = append(errs, &FieldError{Field: "roots", Msg: "must contain at least one entry"})
	}
	for i, root := range cfg.Roots {
		errs = append(errs, validatePath(fmt.Sprintf("roots[%d].path", i), root.Path)...)
		if root.Bucket == "" {
			errs = append(errs, &FieldError{
				Field: fmt.Sprintf("roots[%d].bucket", i),
				Msg:   "required",
			})
		}
	}
	if cfg.Role != RoleWriter && cfg.Role != RoleSpooler {
		errs = append(errs, &FieldError{Field: "role", Msg: `must be "writer" or "spooler"`})
	}
	if cfg.Summarizer != "" && cfg.Summarizer != SummarizerMechanical && cfg.Summarizer != SummarizerOllama {
		errs = append(errs, &FieldError{Field: "summarizer", Msg: `must be "mechanical" or "ollama"`})
	}
	return errors.Join(errs...)
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
