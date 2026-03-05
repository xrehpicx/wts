# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows SemVer.

## [Unreleased]

### Added

- `workswitch` CLI (`wts`, short for worktree switch) with Cobra command surface
- Bubble Tea + Lipgloss TUI for worktree navigation, process selection, and handoff
- Repo-local process profile config loader for `.wts.yaml` with validation/defaults
- Runtime manager enforcing single active worktree process handoff semantics
- Git worktree discovery integration using `git worktree list --porcelain`
- Interactive picker with `fzf` support and built-in fallback prompt
- Generated CLI markdown docs and man pages under `docs/`

### Changed

- Removed state/group model (`~/.workswitch/state.yaml`, group overrides, assignment commands)
- Runtime now discovers worktrees live from Git and preempts only the previously active worktree
- Simplified command set to pure worktree/process handoff operations

## [0.1.0] - 2026-03-05

### Added

- Initial Go CLI scaffold (`hello`, `version`, `help`)
- Makefile workflow with `airflow`, `build`, and quality targets
- Open-source starter docs and GitHub templates
