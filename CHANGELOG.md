# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows SemVer.

## [0.3.0](https://github.com/xrehpicx/wts/compare/v0.2.0...v0.3.0) (2026-03-07)


### Features

* add make release for auto patch bump, tag, and push ([73cee0d](https://github.com/xrehpicx/wts/commit/73cee0dad81931b922813827b8ca3fa70485380f))
* fix version display and add GitHub Actions release workflow ([e46deb8](https://github.com/xrehpicx/wts/commit/e46deb862682ef501fa9944a5ba938ca8ca23df7))
* support multiple processes per worktree via tmux panes ([ae15fbe](https://github.com/xrehpicx/wts/commit/ae15fbe7f25c6a31a100c39aeac9d5bb6a6098b6))


### Bug Fixes

* legacy pane matching and reliable exited detection ([adda249](https://github.com/xrehpicx/wts/commit/adda249accc0eedd639e5bd191c8ba61783f8b4d))
* switch starts only selected process, add vim keybindings ([834e2d8](https://github.com/xrehpicx/wts/commit/834e2d8fd746faa844997677fbe2852a88ee65b3))
* use pgrep to detect exited processes instead of pane command name ([80081a2](https://github.com/xrehpicx/wts/commit/80081a211fe5f7852b16d5917ee1037d5dcb00c1))

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
