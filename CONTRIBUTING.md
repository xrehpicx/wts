# Contributing

Thanks for contributing.

## Development setup

1. Install Go 1.26+, GNU Make, and a POSIX-compatible shell.
2. Clone the repo.
3. Run:

```bash
make airflow
```

## Branching

1. Create a feature branch from `main`.
2. Keep commits focused and small.
3. Open a PR with context, test evidence, and any breaking changes.

## Pull request checklist

- [ ] `make airflow` passes locally
- [ ] Added/updated tests for behavior changes
- [ ] Updated docs (`README.md`/`CHANGELOG.md`) as needed
- [ ] PR title is clear and scoped

## Commit convention (recommended)

Use Conventional Commits, e.g.:

- `feat: add hello --upper flag`
- `fix: handle unknown command exit code`
- `docs: improve make workflow section`

## Verification and publication

Run `make airflow` for formatting, dependency checks, static analysis,
race-tested coverage (minimum 60%), vulnerability scanning, and a production build.
Run `make test-e2e` with Git and tmux installed to verify real process handoff.
Release script tests use temporary Git repositories and a stub GitHub CLI, so they
need no credentials or network. Run `make docs` after changing command help.

Maintainers release a clean, pushed `main` with `make release` (patch) or
`make release BUMP=minor`. The GitHub workflow verifies the requested revision and
publishes binaries and checksums for that exact commit. Check the workflow result
before announcing the release.
