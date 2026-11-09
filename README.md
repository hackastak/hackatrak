# Hackatrak

[![CI](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml/badge.svg)](https://github.com/hackastak/hackatrak/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/hackastak/hackatrak.svg)](https://pkg.go.dev/github.com/hackastak/hackatrak)
[![Go Report Card](https://goreportcard.com/badge/github.com/hackastak/hackatrak)](https://goreportcard.com/report/github.com/hackastak/hackatrak)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Hackatrak** is a local-first, automatic work log. It scans the git
repositories and files you actually touch, collapses that activity into a
per-day record of where your effort went, and keeps everything in a local
SQLite database — no accounts, no servers, nothing leaves your machine.

> **Status:** early development (`0.x`). The `sync` and `version` commands work;
> `today`, `summary`, `log`, and `install` are stubs. Interfaces may change
> before `v1.0.0`.

## How it works

1. You point Hackatrak at the directories where your work lives (via
   `~/.config/hackatrak/config.toml`).
2. `hackatrak sync` walks those trees, reads git history and file modification
   times, and resolves each path to a **project** (nearest git root), a **life
   bucket**, and an Obsidian **wikilink slug**.
3. Activity is collapsed into day-level *presence*, summarized, and written to
   a local store.
4. On a `writer` account, that same run splices one bullet per project into
   today's weekday under that root's bucket in the weekly note. Each auto line
   ends with an invisible `<!-- ht:<slug> -->` marker. A re-run replaces that
   line and leaves unmarked lines alone. A missing week file is created from
   `_templates/Weekly_note.md`. A `spooler` run and `sync --dry-run` do not
   edit the note.

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
hackatrak sync          # scan, record presence, and splice writer bullets
hackatrak sync --dry-run  # print detected work without writing
hackatrak today         # (planned) what you worked on today
hackatrak summary       # (planned) a rollup for a period
hackatrak log           # (planned) recent presence entries
hackatrak version       # print build version and commit
```

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
internal/vault/     Weekly-note path, template fill, and marked-bullet splice
```

## License

[MIT](LICENSE) © Hackastak
