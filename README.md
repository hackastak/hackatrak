# Hackatrak

[![CI](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml/badge.svg)](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/hackastak/hackatrak.svg)](https://pkg.go.dev/github.com/hackastak/hackatrak)
[![Go Report Card](https://goreportcard.com/badge/github.com/hackastak/hackatrak)](https://goreportcard.com/report/github.com/hackastak/hackatrak)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Hackatrak** is a local-first, automatic work log. It scans the git
repositories and files you actually touch, collapses that activity into a
per-day record of where your effort went, and keeps everything in a local
SQLite database — no accounts, no servers, nothing leaves your machine.

> **Status:** early development (`0.x`). `sync`, `today`, `summary`, `log`, `install`,
> `uninstall`, and `version` work. Interfaces may change before `v1.0.0`.

## How it works

1. You point Hackatrak at the directories where your work lives (via
   `~/.config/hackatrak/config.toml`).
2. `hackatrak sync` walks those trees, reads git history and file modification
   times, and resolves each path to a **project** (nearest git root), a **life
   bucket**, and an Obsidian **wikilink slug**.
3. Activity is collapsed into day-level *presence*, summarized, and written to
   a local store.
4. What happens next depends on `role` in the config. A `writer` account (the
   personal Mac) splices one bullet per project into today's weekday under that
   root's bucket in the weekly note. The same run reads current-week spool
   files and splices days labeled `SAP` or `SAP Business Network & ANT` into
   that heading. Each auto line ends with an invisible `<!-- ht:<slug> -->`
   marker. A re-run replaces that line and leaves unmarked lines alone. Spool
   files are not deleted. A missing week file is created from
   `_templates/Weekly_note.md` only when there is a bullet to write.
5. A `spooler` account (the work Mac) writes only
   `spool-<account>-<YYYY>-W<ww>.json` under `spool_dir` (default
   `_Weekly/.hackatrak`). It never opens the weekly note. An unchanged spool
   is left untouched, and an idle week does not replace an existing file.
   `sync --dry-run` prints detected work and writes neither the note nor a
   spool.
6. `hackatrak today` and `hackatrak summary` read the local store. They do
   not parse the weekly note. `hackatrak log` is the other direction: a
   writer-only splice for work that left no git or file footprint. It does
   not insert a store row, so review does not repeat a manual line unless
   that project also has recorded presence.

## Install

### From source

```sh
git clone https://github.com/hackastak/hackatrak.git
cd hackatrak
make install            # installs to ~/.local/bin (override with PREFIX=)
```

Make sure the install target is on your `PATH`, then:

```sh
hackatrak version
```

### Prebuilt binaries

Once releases are published, download the archive for your platform from the
[Releases](https://github.com/hackastak/hackatrak/releases) page, extract it,
and move the `hackatrak` binary onto your `PATH`.

## Usage

```sh
hackatrak sync                         # scan and record; writer splices, spooler writes a spool
hackatrak sync --dry-run               # print detected work without writing the store, note, or spool
hackatrak sync --dry-run --date 2026-11-09  # limit that preview to one local day
hackatrak sync --verbose               # log the day, mode, and scan counts to stderr
hackatrak today                        # table of today's per-project work
hackatrak today --json                 # the same review as JSON
hackatrak summary                      # today's rollup, with a project count
hackatrak summary --week               # current ISO week, grouped by day
hackatrak summary --week --json
hackatrak log "project: did X"          # splice a manual bullet for today
hackatrak log "project (NEXTGEN-1): did X"
hackatrak install                      # load the 12:00 and 23:00 launchd agent (macOS)
hackatrak uninstall                    # unload and remove that agent
hackatrak version                      # print build version and commit
```

Success exits 0. A bad flag, a bad argument, an unknown command, or a bad
`--date` exits 2. Config, scan, and write failures exit 1. Errors go to
stderr. Review text, JSON, and the `sync` summary go to stdout. `--verbose`
and splice notes go to stderr.

### Review

`today` and `summary` read `internal/query` and turn each worked project-day
into a line with the configured summarizer. Unworked rows are omitted. The
table columns are `PROJECT`, `BUCKET`, `SOURCE`, `BRANCH`, `TICKET`, and
`WORK`. An empty cell is `-`. Project is the wikilink slug.

`today` prints the local date, then the table. No work prints `today: no work`.

`summary` is the same day plus a count, `1 project` or `N projects`.
`summary --week` prints the current ISO week (`2026-W46`), then one dated
table per day that has work, then `N projects over D days`. No work prints
`summary: no work`. `--week` does not take a week id. It is always the
current local ISO week.

`--json` replaces the table. `projects` is always an array, never null.
A day document has `period`, `date`, `projects`, `project_count`, and
`day_count`. A week document has `week` instead of `date`.

```json
{
  "period": "day",
  "date": "2026-11-10",
  "projects": [
    {
      "day": "2026-11-10",
      "project": "staksmith",
      "bucket": "",
      "source": "commit",
      "branch": "main",
      "ticket": "NEXTGEN-1",
      "work": "rework docs flow"
    }
  ],
  "project_count": 1,
  "day_count": 1
}
```

`bucket` is empty unless a project row was stored. v1 review does not join
the `projects` table, so the column is usually `-` in the table and `""` in
JSON. `work` is the summary clause only, not the weekly-note bullet.

### Manual log

`log` is for work that left no commit and no file touch. It splices today's
weekday in the weekly note with the same marker `sync` uses, so a later sync
for that project replaces the line instead of duplicating it. A second `log`
for the same project replaces the marked line. Unmarked lines are left alone.
A missing week file is created from the template.

```sh
hackatrak log "staksmith: paired the manual escape hatch"
hackatrak log "staksmith (NEXTGEN-9): replaced the manual line"
```

The project name is the directory basename under a configured root, or a
discovered git repo's basename. That match chooses the bucket. An unknown
project, or the same slug under two buckets, is an error and does not write.
A spooler account is refused: `log` does not open the weekly note and does
not write a spool file.

`log` does not record presence. `today` will not show the manual sentence
unless a later `sync` has store rows for that project. Those rows produce
the summarizer line, which then replaces the manual bullet.

### Sync flags

`--dry-run` prints detected commits and file touches and writes nothing: no
store rows, no cursor advance, no note, no spool.

`--date YYYY-MM-DD` is a local calendar day. Omit it, or pass today, and
`sync` behaves as a normal run. A past day requires `--dry-run`. That preview
is limited to the day's local midnight through the next midnight, so later
commits and file touches are not listed. A future day, or any other shape,
is a usage error. A past `--date` without `--dry-run` is also a usage error,
so a review cannot rewind the scan cursor.

`--verbose` (`-v`) writes three lines to stderr: the day, `mode dry-run` or
`mode write`, and the detected commit and file-touch counts. The stdout
summary is unchanged.

## Scheduled runs (macOS)

Hackatrak is meant to run itself twice a day. `hackatrak install` writes a
per-user LaunchAgent and loads it. The agent runs `sync` at **12:00** and
**23:00** local time.

The plist has no `TimeZone` key, so launchd uses the system timezone and
follows DST. If the Mac is asleep at either time, launchd runs that missed
interval once on wake. Catch-up is a normal `sync`: it records today's work
and splices today's bullets. It does not replay earlier days.

Loading the agent does not run `sync` immediately (`RunAtLoad` is off). Logs:

- `~/Library/Logs/hackatrak/sync.out.log`
- `~/Library/Logs/hackatrak/sync.err.log`

### Install the agent

Install the binary first. `hackatrak install` records the absolute path of
the binary you run, because launchd does not use your shell `PATH`. It also
records that `PATH` so the scheduled job can still find `git`.

```sh
make install          # ~/.local/bin/hackatrak
hackatrak install
```

Check it, and run one sync now:

```sh
launchctl print gui/$(id -u)/com.hackastak.hackatrak
launchctl start com.hackastak.hackatrak
tail -n 40 ~/Library/Logs/hackatrak/sync.out.log
```

Remove it:

```sh
hackatrak uninstall
```

`install` prefers `launchctl bootstrap`. If that subcommand is missing, it
falls back to `launchctl load -w`. A reinstall boots the job out first so
the new plist is the one that is loaded.

### Manual load

The template is
[`internal/launchd/com.hackastak.hackatrak.plist.tmpl`](internal/launchd/com.hackastak.hackatrak.plist.tmpl).
Substitute the absolute binary path, both log paths, and `PATH`, then:

```sh
mkdir -p ~/Library/LaunchAgents ~/Library/Logs/hackatrak
cp /path/to/filled.plist ~/Library/LaunchAgents/com.hackastak.hackatrak.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.hackastak.hackatrak.plist
```

Older macOS:

```sh
launchctl load -w ~/Library/LaunchAgents/com.hackastak.hackatrak.plist
```

Unload without the CLI:

```sh
launchctl bootout gui/$(id -u)/com.hackastak.hackatrak
rm ~/Library/LaunchAgents/com.hackastak.hackatrak.plist
```

`launchctl unload -w ~/Library/LaunchAgents/com.hackastak.hackatrak.plist` is
the legacy equivalent.

## Configuration

Hackatrak reads a TOML config file. Copy the example and edit it:

```sh
mkdir -p ~/.config/hackatrak
cp config.example.toml ~/.config/hackatrak/config.toml
```

See [`config.example.toml`](config.example.toml) for the full set of options
and inline documentation.

## Development

```sh
make build        # build ./hackatrak
make test         # race-enabled test suite
make lint         # golangci-lint (installed locally on first run)
make check        # full local gate: fmt + vet + lint + test
make help         # list all targets
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow, commit
conventions, and release process.

## Project layout

```
cmd/hackatrak/      CLI entry point and command wiring (cobra)
internal/config/    TOML config loading and validation
internal/scan/      Repository and file scanning, project/bucket/slug resolution
internal/derive/    Deriving presence from raw activity
internal/summarize/ Mechanical day-level summarization
internal/store/     SQLite persistence and migrations
internal/query/     Today / week queries
internal/spool/     Cross-account spool JSON: write, read, and discover
internal/vault/     Weekly-note path, template fill, and marked-bullet splice
internal/launchd/   LaunchAgent plist template and launchctl load/unload
```

## License

[MIT](LICENSE) © Hackastak
