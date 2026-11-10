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
   bucket**, and an Obsidian **wikilink slug**. A file touch stored on a scan
   root that is not itself a project is skipped and logged. It does not abort
   the run, and it does not become a bullet.
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
summary is unchanged. A skipped scan root is also logged to stderr, with or
without `--verbose`: `sync: skip <path>: no project`.

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

Hackatrak reads one TOML file, `~/.config/hackatrak/config.toml`. `~` in the
config path and in path values is expanded. Unknown keys are an error, and a
parse or validation error names the field and, when the parser knows it, the
source line. There is no `--config` flag. One file per macOS account.

The SQLite store is not in this file. It is always
`~/.local/state/hackatrak/hackatrak.db`, created on the first command that
opens it. Keep that file local. Do not put it in the vault.

```sh
mkdir -p ~/.config/hackatrak
cp config.example.toml ~/.config/hackatrak/config.toml
```

[`config.example.toml`](config.example.toml) is the same reference with the
comments inline. The personal account is the writer. The work account is the
spooler and must not set `role` to `"writer"`.

### Fields

| Field | Required | Default | Meaning |
| --- | --- | --- | --- |
| `vault_path` | yes | none | Obsidian vault. Must be an existing directory. |
| `weekly_glob` | no | `_Weekly/{YYYY}/{YYYY}-W{ww}.md` | Week-file path relative to the vault. `{YYYY}` and `{ww}` are filled when the week is resolved, not at load. |
| `account` | yes | none | This macOS account. One filename segment: letters, digits, `.`, `_`, `-`. It must not contain `..`. Used in `spool-<account>-<YYYY>-W<ww>.json`. |
| `role` | yes | none | `writer` or `spooler`. |
| `spool_dir` | no | `_Weekly/.hackatrak` | Cross-account spool directory. It must sit inside the vault, must not contain `..`, and does not need to exist yet. A relative path is joined to `vault_path`. `~` is expanded. |
| `ticket_regex` | no | none | Tried against the branch name, then the commit message. No match omits the parentheses in the bullet. |
| `exclude_globs` | no | none | Extra mtime-scan ignores, on top of the built-ins below. |
| `summarizer` | no | `mechanical` | `mechanical` or `ollama`. `ollama` is accepted and returns an error until that summarizer is implemented. |
| `roots` | yes | none | One or more `{ path, bucket }` tables. |
| `roots[].path` | yes | none | Existing directory. Only this account's trees. |
| `roots[].bucket` | yes | none | Weekly-note heading for that path prefix. |
| `git.author_emails` | no | none | Commits by other addresses are ignored. An empty list records no commits. |

Built-in mtime excludes, always on, are `.git/`, build-output directories
(`node_modules`, `vendor`, `dist`, `build`, `target`, `coverage`, `.next`,
`.turbo`, `__pycache__`), `*.swp`, `*~`, `.DS_Store`, `*.o`, `*.pyc`, and
`*.class`. A pattern in `exclude_globs` that contains `/` matches the path
relative to the walk root. Any other pattern matches the file or directory
name. `*` does not cross a separator. `.gitignore` files are always honored
and are not a config field.

`role = "writer"` splices one marked bullet per project into today's weekday
under that root's bucket. The same run reads every current-week spool file
and splices a day whose bucket is `SAP` or `SAP Business Network & ANT` into
the `SAP Business Network & ANT` heading. Any other spool bucket is logged
and skipped. Spool files are not deleted. Only the writer opens the weekly
note.

`role = "spooler"` writes only its own spool file and never opens the note.
An unchanged spool is left untouched. An idle week does not replace an
existing spool.

### Personal account (`hackastak`)

```toml
vault_path = "~/Developer/My_Notes"
account = "hackastak"
role = "writer"
spool_dir = "_Weekly/.hackatrak"
ticket_regex = 'NEXTGEN-\\d+'
summarizer = "mechanical"

[[roots]]
path = "~/Developer"
bucket = "Hackastak"

[git]
author_emails = ["you@example.com"]
```

### Work account (`i568439`)

Copy this to `~/.config/hackatrak/config.toml` on the work Mac, then
`make install` and `hackatrak install`. Do not install this role on the
personal account.

```toml
vault_path = "~/Developer/My_Notes"
account = "i568439"
role = "spooler"
spool_dir = "_Weekly/.hackatrak"
ticket_regex = 'NEXTGEN-\\d+'
summarizer = "mechanical"

[[roots]]
path = "~/Developer"
bucket = "SAP Business Network & ANT"

[git]
author_emails = ["you@example.com"]
```

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
