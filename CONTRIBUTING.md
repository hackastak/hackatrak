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

### Future: Homebrew distribution

A Homebrew tap (`brew install`) is intentionally **not** set up yet. It's a
public "stable enough to install and keep updated" promise, so it's deferred
until the MVP commands work and there's a release worth telling people to
install (roughly the first functional `v0.1.0`+). When that time comes, it's a
small addition on top of the existing release pipeline:

1. **Create the tap repository.** A public repo named `homebrew-tap` under the
   same owner (`github.com/hackastak/homebrew-tap`). GoReleaser commits the
   generated formula here; users then run
   `brew install hackastak/tap/hackatrak`.
2. **Create a push token.** A fine-grained personal access token (or a machine
   user's token) with **contents: write** on the tap repo. Add it to the
   `hackatrak` repo as an Actions secret, e.g. `HOMEBREW_TAP_GITHUB_TOKEN`.
3. **Expose the secret to the release job** in
   [`.github/workflows/release.yml`](.github/workflows/release.yml), alongside
   the existing `GITHUB_TOKEN`:

   ```yaml
   env:
     GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
     HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
   ```

4. **Add a `brews:` block** to [`.goreleaser.yaml`](.goreleaser.yaml):

   ```yaml
   brews:
     - name: hackatrak
       repository:
         owner: hackastak
         name: homebrew-tap
         token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
       homepage: https://github.com/hackastak/hackatrak
       description: Local-first, automatic work log
       license: MIT
       test: |
         system "#{bin}/hackatrak", "version"
   ```

5. **Validate and release.** Run `goreleaser check`, then cut a tag as usual.
   GoReleaser builds the binaries, pushes the formula to the tap, and the
   `brew install` path goes live.

Keep the formula's `license` and `description` in sync with
[`LICENSE`](LICENSE) and the project description if they change.

## Code of conduct

By participating you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).
