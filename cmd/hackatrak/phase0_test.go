package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // register the pure-Go driver
)

// TestPhase0 is the Phase 0 acceptance gate (P0.T): version runs, a malformed
// config fails with field and line, and the first run creates the database
// with the full schema.
func TestPhase0(t *testing.T) {
	bin := buildHackatrak(t)

	t.Run("version", func(t *testing.T) {
		home := t.TempDir()
		out, err := runHackatrak(t, bin, home, "version")
		if err != nil {
			t.Fatalf("hackatrak version: %v\n%s", err, out)
		}
		if !strings.Contains(out, "hackatrak ") || strings.Contains(out, "not implemented") {
			t.Fatalf("version output = %q", out)
		}
		if _, err := os.Stat(defaultDB(home)); !os.IsNotExist(err) {
			t.Fatalf("version created a database: %v", err)
		}
	})

	t.Run("malformed config", func(t *testing.T) {
		home := t.TempDir()
		body := fmt.Sprintf("vault_path = %q\nrole = \"nope\"\naccount = \"hackastak\"\n\n[[roots]]\npath = %q\nbucket = \"Hackastak\"\n", home, home)
		writeConfigBody(t, home, body)

		out, err := runHackatrak(t, bin, home, "sync")
		if err == nil {
			t.Fatalf("sync with malformed config succeeded: %s", out)
		}
		if !strings.Contains(out, "role at line 2") {
			t.Fatalf("output = %q, want field and line", out)
		}
		if _, statErr := os.Stat(defaultDB(home)); !os.IsNotExist(statErr) {
			t.Fatalf("malformed config created a database: %v", statErr)
		}
	})

	t.Run("first run schema", func(t *testing.T) {
		home := t.TempDir()
		writeValidConfig(t, home)
		dbPath := defaultDB(home)
		if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
			t.Fatalf("database exists before first run: %v", err)
		}

		out, err := runHackatrak(t, bin, home, "sync")
		if err != nil {
			t.Fatalf("first run: %v\n%s", err, out)
		}
		if _, err := os.Stat(dbPath); err != nil {
			t.Fatalf("first run did not create %s: %v", dbPath, err)
		}
		assertFullSchema(t, dbPath)

		out, err = runHackatrak(t, bin, home, "sync")
		if err != nil {
			t.Fatalf("second run: %v\n%s", err, out)
		}
		assertFullSchema(t, dbPath)
	})
}

func buildHackatrak(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "hackatrak")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runHackatrak(t *testing.T, bin, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = envWithHome(home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func envWithHome(home string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if strings.HasPrefix(entry, "HOME=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+home)
}

func writeConfigBody(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "hackatrak")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func defaultDB(home string) string {
	return filepath.Join(home, ".local", "state", "hackatrak", "hackatrak.db")
}

func assertFullSchema(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
	}

	tables := queryStrings(t, db, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	wantTables := []string{"day_presence", "events", "projects", "sync_state"}
	if strings.Join(tables, ",") != strings.Join(wantTables, ",") {
		t.Fatalf("tables = %v, want %v", tables, wantTables)
	}

	wantCols := map[string][]string{
		"projects": {"id", "slug", "name", "root_path", "bucket"},
		"events": {
			"id", "kind", "hash", "repo_path", "project_slug", "occurred_at",
			"branch", "subject", "body", "file_count", "file_sample",
		},
		"day_presence": {
			"id", "project_slug", "day", "worked", "source", "branch", "ticket",
			"commit_hashes", "file_count", "file_sample",
		},
		"sync_state": {"repo_path", "cursor", "updated_at"},
	}
	for table, cols := range wantCols {
		got := queryStrings(t, db, fmt.Sprintf("SELECT name FROM pragma_table_info(%q) ORDER BY cid", table))
		if strings.Join(got, ",") != strings.Join(cols, ",") {
			t.Fatalf("%s columns = %v, want %v", table, got, cols)
		}
	}

	wantSQL := map[string][]string{
		"projects": {
			"id INTEGER PRIMARY KEY",
			"slug TEXT NOT NULL UNIQUE",
			"name TEXT NOT NULL",
			"root_path TEXT NOT NULL",
			"bucket TEXT NOT NULL DEFAULT ''",
		},
		"events": {
			"kind TEXT NOT NULL",
			"hash TEXT",
			"repo_path TEXT NOT NULL",
			"project_slug TEXT NOT NULL DEFAULT ''",
			"occurred_at TEXT NOT NULL",
			"file_count INTEGER NOT NULL DEFAULT 0",
			"file_sample TEXT NOT NULL DEFAULT ''",
		},
		"day_presence": {
			"project_slug TEXT NOT NULL",
			"day TEXT NOT NULL",
			"worked INTEGER NOT NULL",
			"source TEXT NOT NULL",
			"UNIQUE(project_slug, day)",
		},
		"sync_state": {
			"repo_path TEXT PRIMARY KEY",
			"cursor TEXT NOT NULL",
			"updated_at TEXT NOT NULL",
		},
		"events_hash": {
			"CREATE UNIQUE INDEX events_hash ON events(hash) WHERE hash IS NOT NULL",
		},
		"events_repo_time": {
			"CREATE INDEX events_repo_time ON events(repo_path, occurred_at)",
		},
	}
	for name, fragments := range wantSQL {
		sqlText := masterSQL(t, db, name)
		normalized := strings.Join(strings.Fields(sqlText), " ")
		for _, fragment := range fragments {
			if !strings.Contains(normalized, strings.Join(strings.Fields(fragment), " ")) {
				t.Errorf("%s schema missing %q\n%s", name, fragment, sqlText)
			}
		}
	}
}

func masterSQL(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var sqlText string
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&sqlText)
	if err != nil {
		t.Fatalf("schema for %s: %v", name, err)
	}
	return sqlText
}

func queryStrings(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
