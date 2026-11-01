package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "state", "hackatrak", DefaultDBName)
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("Open(empty) error = nil, want an error")
	}
}

func TestOpenRejectsOtherUserTilde(t *testing.T) {
	if _, err := Open("~other/hackatrak.db"); err == nil {
		t.Fatal("Open(~other) error = nil, want an error")
	}
}

func TestCloseNil(t *testing.T) {
	var db *DB
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := expandHome("~/.local/state/hackatrak/hackatrak.db")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "state", "hackatrak", "hackatrak.db")
	if got != want {
		t.Fatalf("expandHome(~/...) = %q, want %q", got, want)
	}

	if _, err := expandHome("~other/hackatrak.db"); err == nil {
		t.Fatal("expandHome(~other) error = nil, want an error")
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hackatrak.db")

	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })

	version, err := second.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hackatrak.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(path); err == nil {
		t.Fatal("Open(newer schema) error = nil, want an error")
	}
}

func TestOpenCreatesDBAndSchemaV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "hackatrak.db")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file: %v", err)
	}
	if db.Path() != path {
		t.Fatalf("Path() = %q, want %q", db.Path(), path)
	}

	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}

	got, err := db.tableNames()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"day_presence", "events", "projects", "sync_state"}
	if len(got) != len(want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tables = %v, want %v", got, want)
		}
	}
}
