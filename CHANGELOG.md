# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the project is in initial development (`0.x`), the public interface may
change at any time. This log records notable, user-facing changes — not every
commit. Day-to-day implementation detail lives in the git history.

## [Unreleased]

The work below is the initial build, accumulating toward the first tagged
release (`v0.1.0`). Until then it all lives under _Unreleased_.

### Added

- Local-first work-log core backed by SQLite (`modernc.org/sqlite`, no CGo).
- `sync` command: scans configured repositories, collapses git and file
  activity into per-day presence, and persists it to the local store.
- Repository scanning: resolves a project from the nearest git root (falling
  back to the first path segment), derives the life bucket and Obsidian
  wikilink slug from the project path, and tracks a cursor so each run only
  processes new activity.
- Query layer for "today" and "this week" presence, including derived rows and
  file-only days.
- Mechanical summarization of collected activity into day-level presence.
- TOML configuration with schema validation (`config.example.toml`).
- `version` command reporting the build version and git commit, injected at
  build time via `-ldflags`.
- `today`, `summary`, `log`, and `install` command stubs (not yet implemented).

### Project infrastructure

- MIT license.
- Continuous integration (build, `go vet`, `golangci-lint`, race-enabled tests
  with coverage).
- Release pipeline via GoReleaser (cross-platform binaries on tag push).
- Dependabot for Go modules and GitHub Actions.
- Contributor, security, and code-of-conduct documentation.

[Unreleased]: https://github.com/hackastak/hackatrak/commits/main
