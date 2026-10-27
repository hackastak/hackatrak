package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsEmptyRootsAndBadRole(t *testing.T) {
	dir := t.TempDir()
	err := Validate(Config{
		VaultPath: dir,
		Role:      "admin",
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}

	var fields []*FieldError
	for _, part := range flatten(err) {
		var fe *FieldError
		if errors.As(part, &fe) {
			fields = append(fields, fe)
		}
	}
	seen := map[string]string{}
	for _, fe := range fields {
		seen[fe.Field] = fe.Error()
	}
	if _, ok := seen["roots"]; !ok {
		t.Errorf("errors = %v, want roots", err)
	}
	if _, ok := seen["role"]; !ok {
		t.Errorf("errors = %v, want role", err)
	}
}

func TestValidatePathsExist(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Validate(Config{
		VaultPath: missing,
		Role:      RoleWriter,
		Roots:     []Root{{Path: filePath, Bucket: "Hackastak"}},
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	msg := err.Error()
	if !hasField(err, "vault_path") {
		t.Errorf("error = %q, want vault_path", msg)
	}
	if !hasField(err, "roots[0].path") {
		t.Errorf("error = %q, want roots[0].path", msg)
	}
}

func TestValidateRejectsUnknownSummarizer(t *testing.T) {
	dir := t.TempDir()
	err := Validate(Config{
		VaultPath:  dir,
		Role:       RoleWriter,
		Roots:      []Root{{Path: dir, Bucket: "Hackastak"}},
		Summarizer: "gpt",
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if !hasField(err, "summarizer") {
		t.Errorf("error = %q, want summarizer", err)
	}
}

func TestValidateAcceptsExistingDirectories(t *testing.T) {
	dir := t.TempDir()
	err := Validate(Config{
		VaultPath:  dir,
		Role:       RoleSpooler,
		Roots:      []Root{{Path: dir, Bucket: "Hackastak"}},
		Summarizer: SummarizerMechanical,
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func hasField(err error, field string) bool {
	for _, part := range flatten(err) {
		var fe *FieldError
		if errors.As(part, &fe) && fe.Field == field {
			return true
		}
	}
	return false
}

func flatten(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, part := range joined.Unwrap() {
			out = append(out, flatten(part)...)
		}
		return out
	}
	return []error{err}
}
