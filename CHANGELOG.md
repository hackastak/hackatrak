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

- `hackatrak install` writes a per-user LaunchAgent
  (`~/Library/LaunchAgents/com.hackastak.hackatrak.plist`) that runs `sync`
  at 12:00 and 23:00 local time, then loads it with `launchctl bootstrap`
  (legacy `launchctl load -w` fallback). Logs go to
  `~/Library/Logs/hackatrak/`. A missed fire runs once on wake. The scheduled
  program is the absolute path of the binary you install, not a bare name.
- `hackatrak uninstall` unloads that agent and removes its plist.
- Local-first work-log core backed by SQLite (`modernc.org/sqlite`, no CGo).
- `sync` command: scans configured repositories, collapses git and file
  activity into per-day presence, and persists it to the local store.
- Writer-role `sync` splices one marked bullet per project into today's
  weekday of that root's weekly-note bucket. It also splices current-week
  spool days labeled `SAP` or `SAP Business Network & ANT` into that heading
  and skips any other spool heading. A re-run replaces the marked line and
  leaves hand-written lines alone. Spool files are left in place. A missing
  week file is created from `_templates/Weekly_note.md` only when there is
  something to write. `--dry-run` does not edit the note.
- Spooler-role `sync` writes this account's ISO week to
  `spool-<account>-<YYYY>-W<ww>.json` under `spool_dir` (default
  `_Weekly/.hackatrak`) and never opens the weekly note. An unchanged file
  is not rewritten. An idle week leaves an existing spool where it is.
- Repository scanning: resolves a project from the nearest git root (falling
  back to the first path segment), derives the life bucket and Obsidian
  wikilink slug from the project path, and tracks a cursor so each run only
  processes new activity.
- Query layer for "today" and "this week" presence, including derived rows and
  file-only days.
- Mechanical summarizer: condenses commit subjects into one line, with a
  file-touch fallback and optional ticket extraction. The Ollama summarizer
  is accepted by config and still a stub.
- TOML configuration with schema validation (`config.example.toml`).
- `version` command reporting the build version and git commit, injected at
  build time via `-ldflags`.
- `hackatrak today` prints a table of today's per-project work from the
  query layer. `hackatrak summary` rolls up that day, and `summary --week`
  rolls up the current ISO week. Both accept `--json`. An idle period prints
  `today: no work` or `summary: no work` instead of an empty table.
- `hackatrak log "project: did X"` splices a manual bullet for today on the
  writer account, using the same `<!-- ht:<slug> -->` marker as `sync`.
  `project (TICKET): did X` adds the ticket parentheses. The project must be
  a directory under a configured root. A spooler run is refused and does not
  open the note. A second log for the same project replaces the line. The
  manual line is not written to the store, so `today` does not repeat it.
- `sync --date YYYY-MM-DD` previews one local day when combined with
  `--dry-run`. A past date without `--dry-run`, a future date, or a malformed
  date is a usage error. `--verbose` (`-v`) logs the day, mode, and scan
  counts to stderr.
- Usage errors (unknown command or flag, bad arguments, bad `--date`) exit 2.
  Config, scan, and write failures exit 1. Success exits 0.

### Project infrastructure

- MIT license.
- Continuous integration (build, `go vet`, `golangci-lint`, race-enabled tests
  with coverage).
- Release pipeline via GoReleaser (cross-platform binaries on tag push).
- Dependabot for Go modules and GitHub Actions.
- Contributor, security, and code-of-conduct documentation.

[Unreleased]: https://github.com/hackastak/hackatrak/commits/main
