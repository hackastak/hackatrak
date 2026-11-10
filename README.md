# Hackatrak

[![CI](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml/badge.svg)](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/hackastak/hackatrak.svg)](https://pkg.go.dev/github.com/hackastak/hackatrak)
[![Go Report Card](https://goreportcard.com/badge/github.com/hackastak/hackatrak)](https://goreportcard.com/report/github.com/hackastak/hackatrak)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Hackatrak** is a local-first, automatic work log. It scans the git
repositories and files you actually touch, collapses that activity into a
per-day record of where your effort went, and keeps everything in a local
SQLite database — no accounts, no servers, nothing leaves your machine.

> **Status:** early development (`0.x`). The `sync`, `install`, `uninstall`, and
> `version` commands work; `today`, `summary`, and `log` are stubs. Interfaces
> may change before `v1.0.0`.

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
hackatrak sync          # scan and record; writer splices, spooler writes a spool
hackatrak sync --dry-run  # print detected work without writing the store, note, or spool
hackatrak install       # load the 12:00 and 23:00 launchd agent (macOS)
hackatrak uninstall     # unload and remove that agent
hackatrak today         # (planned) what you worked on today
hackatrak summary       # (planned) a rollup for a period
hackatrak log           # (planned) recent presence entries
hackatrak version       # print build version and commit
```

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
