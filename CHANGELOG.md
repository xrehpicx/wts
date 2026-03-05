# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows SemVer.

## [Unreleased]

### Added

- New `wks` tmux-backed CLI with commands: `list`, `switch`, `start`, `restart`,
  `stop`, `status`, `logs`, `pick`, `validate`, and `version`
- Repo-local config loader for `.workswitch.yaml` with validation and defaults
- Runtime manager enforcing one active workspace process per group
- Interactive picker with `fzf` support and built-in fallback prompt
- Unit/integration-style tests for config normalization, tmux naming, picker fallback,
  and group preemption behavior

## [0.1.0] - 2026-03-05

### Added

- Initial Go CLI scaffold (`hello`, `version`, `help`)
- Makefile workflow with `airflow`, `build`, and quality targets
- Open-source starter docs and GitHub templates
