# Security Policy

## Supported versions

Hackatrak is in initial development (`0.x`). Security fixes are applied to the
latest release and `main` only.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report privately using GitHub's
[private vulnerability reporting](https://github.com/hackastak/hackatrak/security/advisories/new)
("Report a vulnerability" under the repository's **Security** tab).

Please include:

- A description of the issue and its impact.
- Steps to reproduce or a proof of concept.
- Affected version (`hackatrak version`) and platform.

You can expect an initial acknowledgement within a few days. Once a fix is
ready, we'll coordinate a release and credit you in the advisory unless you
prefer to remain anonymous.

## Scope

Hackatrak is a local-first tool that reads local git repositories and files.
It writes a per-account SQLite store, and, when configured, a weekly note or
spool files inside the Obsidian vault. Reports that involve handling of local
data, path traversal, or command execution are in scope.
