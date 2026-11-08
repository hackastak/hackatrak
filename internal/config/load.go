package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultPath is ~/.config/hackatrak/config.toml with ~ already expanded.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config path: %w", err)
	}
	return filepath.Join(home, ".config", "hackatrak", "config.toml"), nil
}

// Load reads a TOML config, expands ~ in path fields, applies defaults, and
// validates the result. A parse error includes the file path and source line.
func Load(path string) (Config, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	data, err := os.ReadFile(expanded)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", expanded, err)
	}

	var cfg Config
	src := string(data)
	md, err := toml.Decode(src, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", expanded, diagnose(err))
	}
	if unknown := unknownFields(md); len(unknown) > 0 {
		return Config{}, fmt.Errorf("load config %s: %w", expanded, withLines(errors.Join(unknown...), src))
	}

	cfg, err = expandConfig(cfg)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", expanded, err)
	}
	cfg = applyDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", expanded, withLines(err, src))
	}
	return cfg, nil
}

// diagnose turns a TOML parse error into a field/line diagnostic.
func diagnose(err error) error {
	var pe toml.ParseError
	if !errors.As(err, &pe) {
		return err
	}
	field := pe.LastKey
	if field == "" {
		field = "config"
	}
	return &FieldError{Field: field, Line: pe.Position.Line, Msg: pe.Message}
}

func withLines(err error, src string) error {
	if err == nil {
		return nil
	}
	parts := flattenErrors(err)
	out := make([]error, 0, len(parts))
	for _, part := range parts {
		var fe *FieldError
		if !errors.As(part, &fe) {
			out = append(out, part)
			continue
		}
		line := fe.Line
		if line == 0 {
			line = lineForField(src, fe.Field)
		}
		out = append(out, &FieldError{Field: fe.Field, Line: line, Msg: fe.Msg})
	}
	return errors.Join(out...)
}

func flattenErrors(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, part := range joined.Unwrap() {
			out = append(out, flattenErrors(part)...)
		}
		return out
	}
	return []error{err}
}

func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return "", fmt.Errorf("expand %q: only the current user's ~ is supported", path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand home: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func expandConfig(cfg Config) (Config, error) {
	vault, err := expandHome(cfg.VaultPath)
	if err != nil {
		return Config{}, fmt.Errorf("vault_path: %w", err)
	}
	spool, err := expandHome(cfg.SpoolDir)
	if err != nil {
		return Config{}, fmt.Errorf("spool_dir: %w", err)
	}
	roots := make([]Root, len(cfg.Roots))
	for i, root := range cfg.Roots {
		expanded, err := expandHome(root.Path)
		if err != nil {
			return Config{}, fmt.Errorf("roots[%d].path: %w", i, err)
		}
		roots[i] = Root{Path: expanded, Bucket: root.Bucket}
	}
	emails := append([]string(nil), cfg.Git.AuthorEmails...)
	globs := append([]string(nil), cfg.ExcludeGlobs...)
	return Config{
		VaultPath:    vault,
		WeeklyGlob:   cfg.WeeklyGlob,
		Roots:        roots,
		Account:      cfg.Account,
		Role:         cfg.Role,
		SpoolDir:     spool,
		Git:          Git{AuthorEmails: emails},
		TicketRegex:  cfg.TicketRegex,
		ExcludeGlobs: globs,
		Summarizer:   cfg.Summarizer,
	}, nil
}

func applyDefaults(cfg Config) Config {
	weekly := cfg.WeeklyGlob
	if weekly == "" {
		weekly = DefaultWeeklyGlob
	}
	summarizer := cfg.Summarizer
	if summarizer == "" {
		summarizer = SummarizerMechanical
	}
	cfg.WeeklyGlob = weekly
	cfg.Summarizer = summarizer
	return cfg
}

func unknownFields(md toml.MetaData) []error {
	var errs []error
	for _, key := range md.Undecoded() {
		errs = append(errs, &FieldError{Field: key.String(), Msg: "unknown field"})
	}
	return errs
}
