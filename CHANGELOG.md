# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows SemVer.

## [Unreleased]

## [0.2.0] - 2026-03-05

### Added

- `wts init` command with built-in detectors (Node.js, Go, Python, Makefile) and custom YAML detectors
- `wts version` now shows commit hash (works with `go install` and `make build`)
- Exited process detection — TUI and `wts status` distinguish running, exited (shell still open), and stopped
- `wts list` aliased to `wts ls`
- CLI feedback: `wts switch/start/restart/stop/next/prev` print `✓` confirmation on success
- Visual status indicators in `wts status` output (`●`/`○`/`★` instead of `true`/`false`)
- Actionable error messages with hints (e.g. `git worktree add` when no worktrees found)
- TUI process search documented in help (`/` key)

### Changed

- Removed state/group model (`~/.workswitch/state.yaml`, group overrides, assignment commands)
- Runtime now discovers worktrees live from Git and preempts only the previously active worktree
- Simplified command set to pure worktree/process handoff operations
- TUI no longer auto-selects a process on open — requires explicit `←`/`→` selection
- Process list ordered by last-used (active process first, then config order) instead of alphabetical
- Navigating worktrees in TUI no longer changes the selected process
- Narrower left panel in TUI (25% instead of 30%) for more detail space
- Spinner changed from braille dots to quarter-circle rotation (`◒◐◓◑`)
- Process names now allow `:`, `@`, `/`, and spaces (for npm script names like `auth:generate`)

## [0.1.0] - 2026-03-05

### Added

- Initial Go CLI scaffold (`hello`, `version`, `help`)
- Makefile workflow with `airflow`, `build`, and quality targets
- Open-source starter docs and GitHub templates
