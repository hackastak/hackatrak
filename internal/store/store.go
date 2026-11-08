// Package store opens the local SQLite database and persists events and day presence.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // register the pure-Go driver
)

// DefaultDBName is the file opened under the per-account state directory.
const DefaultDBName = "hackatrak.db"

// DB is an open Hackatrak database. It is safe for concurrent use.
type DB struct {
	sql  *sql.DB
	path string
}

// DefaultPath is ~/.local/state/hackatrak/hackatrak.db with ~ already expanded.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store path: %w", err)
	}
	return filepath.Join(home, ".local", "state", "hackatrak", DefaultDBName), nil
}

// Open creates parent directories, opens SQLite at path, and runs migrations.
// A leading ~ is expanded to the current user's home directory.
func Open(path string) (*DB, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if expanded == "" {
		return nil, fmt.Errorf("open store: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return nil, fmt.Errorf("open store %s: %w", expanded, err)
	}

	sqlDB, err := sql.Open("sqlite", sqliteDSN(expanded))
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", expanded, err)
	}
	sqlDB.SetMaxOpenConns(1)

	db := &DB{sql: sqlDB, path: expanded}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open store %s: %w", expanded, err)
	}
	if err := migrate(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open store %s: %w", expanded, err)
	}
	return db, nil
}

// Path is the expanded filesystem path of the database file.
func (db *DB) Path() string { return db.path }

// Close releases the database.
func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	if err := db.sql.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}
	return nil
}

// SchemaVersion is the applied migration version (PRAGMA user_version).
func (db *DB) SchemaVersion() (int, error) {
	version, err := schemaVersion(db.sql)
	if err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	return version, nil
}

func (db *DB) tableNames() ([]string, error) {
	rows, err := db.sql.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func sqliteDSN(path string) string {
	// modernc accepts a file path. Pragmas apply on every connection.
	return path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
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
