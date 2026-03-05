# Contributing

Thanks for contributing.

## Development setup

1. Install Go 1.22+.
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
