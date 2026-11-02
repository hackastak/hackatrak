# Hackatrak

Local-first, automatic work log (Go). This repo is a bare scaffold.

The full design, phased build plan, and task list live in the Obsidian vault
under `1. Projects/Hackatrak` (Feature_Brainstorm, Build_Plan, Implementation_Todo).

## Build

```sh
make build    # ./hackatrak
make test
make install  # $(HOME)/.local/bin/hackatrak; override with PREFIX=
make clean
```

## Run

```sh
make build && ./hackatrak version
```
