# Contributing to Hackatrak

Thanks for your interest in contributing. This document covers how to set up
your environment, the quality bar, and how changes get released.

## Prerequisites

- Go (see the version pinned in [`go.mod`](go.mod))
- `make`
- Optional: [`golangci-lint`](https://golangci-lint.run) and
  [`goreleaser`](https://goreleaser.com) for linting and release dry-runs.
  `make lint` will install `golangci-lint` into `./bin` if it isn't on your PATH.

## Getting started

```sh
git clone https://github.com/hackastak/hackatrak.git
cd hackatrak
make build        # builds ./hackatrak
make test         # runs the test suite
./hackatrak version
```

## Development workflow

1. Create a branch off `main`.
2. Write tests first, then the implementation. New code is expected to come
   with tests.
3. Run the full local gate before pushing:

   ```sh
   make check      # fmt + vet + lint + test
   ```

4. Open a pull request. CI runs build, vet, lint, and race-enabled tests on
   Linux, macOS, and Windows; all must pass.

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`.
`feat` and `fix` commits are surfaced in release notes; use `feat!:` or a
`BREAKING CHANGE:` footer for incompatible changes.

## Changelog

[`CHANGELOG.md`](CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com).
Add an entry under `[Unreleased]` for any **user-facing** change. Internal
refactors and test-only changes don't need an entry — the git history covers
those.

## Releasing

Releases are automated with GoReleaser. A maintainer cuts a release by pushing
a SemVer tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

This triggers the release workflow, which builds cross-platform binaries and
publishes a GitHub Release. Before tagging, move the `[Unreleased]` entries
into a new versioned section in the changelog.

## Code of conduct

By participating you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).
