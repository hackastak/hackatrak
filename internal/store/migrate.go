package store

import (
	"database/sql"
	"fmt"
)

// schemaV1 is the version this binary knows how to create.
const schemaV1 = 1

const schemaV1SQL = `
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    root_path TEXT NOT NULL,
    bucket TEXT NOT NULL DEFAULT ''
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,
    hash TEXT,
    repo_path TEXT NOT NULL,
    project_slug TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    file_count INTEGER NOT NULL DEFAULT 0,
    file_sample TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX events_hash ON events(hash) WHERE hash IS NOT NULL;
CREATE INDEX events_repo_time ON events(repo_path, occurred_at);

CREATE TABLE day_presence (
    id INTEGER PRIMARY KEY,
    project_slug TEXT NOT NULL,
    day TEXT NOT NULL,
    worked INTEGER NOT NULL,
    source TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    ticket TEXT NOT NULL DEFAULT '',
    commit_hashes TEXT NOT NULL DEFAULT '',
    file_count INTEGER NOT NULL DEFAULT 0,
    file_sample TEXT NOT NULL DEFAULT '',
    UNIQUE(project_slug, day)
);

CREATE TABLE sync_state (
    repo_path TEXT PRIMARY KEY,
    cursor TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
`

func migrate(sqlDB *sql.DB) error {
	version, err := schemaVersion(sqlDB)
	if err != nil {
		return err
	}
	if version == schemaV1 {
		return nil
	}
	if version > schemaV1 {
		return fmt.Errorf("schema v%d is newer than this binary (v%d)", version, schemaV1)
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	if _, err := tx.Exec(schemaV1SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply schema v1: %w", err)
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaV1)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("set schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func schemaVersion(sqlDB *sql.DB) (int, error) {
	var version int
	if err := sqlDB.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}
