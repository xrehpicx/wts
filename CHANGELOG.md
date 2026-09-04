# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project follows SemVer.

## [Unreleased]

## [0.4.0] - 2026-09-05

### Improved

- Compact worktree lists on narrow terminals, full-area target search and group
  editing, readable secondary text, consistent selection highlighting, and
  context-specific keyboard help
- Selected worktree and target are explicit in the header; errors have a dedicated
  row, target changes refresh logs immediately, and group logs use spare space
- Regression tests for editor state, stale asynchronous replies, terminal bounds,
  shell programs, picker cancellation, unusual paths, and release retries
- Real Git and isolated tmux tests, including compound commands and pane identity;
  raised the enforced total coverage floor from 40% to 60%
- Split initialization and TUI editing into focused modules; preserve command
  cancellation, caller-provided streams, and output errors

### Added

- Cross-platform push and pull-request CI with race-tested coverage, linting,
  vulnerability scanning, dependency checks, and production builds
- Regression tests for process restart, tmux failures, unusual worktree paths,
  detector validation, safe configuration writes, and responsive TUI rendering

### Changed

- Updated the project to Go 1.26 and the stable Bubble Tea, Bubbles, and Lip Gloss
  v2 module families
- Improved the TUI with responsive narrow layouts, terminal-aware colors,
  Unicode-safe truncation, clearer target state, and safer asynchronous refreshes
- Hardened configuration, detector, tmux, runtime, and release error handling

### Fixed

- Compound shell commands, loops, and assignments execute completely instead of
  being cut short by an implicit `exec`
- Graceful shutdown interrupts live child processes even when tmux reports a shell;
  pane metadata takes precedence over mutable titles
- Background refreshes cannot overwrite newer TUI results or mutate the current
  worktree inventory; group saves cannot be submitted twice
- Search can select every matching target and preserves the prior target when no
  result is chosen; `ctrl+c` works in editors
- Picker cancellation no longer opens a fallback prompt; EOF input and paths with
  whitespace or control characters are handled correctly
- Bare and prunable worktrees are rejected before stopping the active worktree;
  invalid NUL execution values are rejected during configuration validation
- Makefile detection follows GNU Make file precedence and skips variable assignments;
  Django detection honors uv and Poetry environments
- Release requests now publish assets, pin the tested commit and exact tag, ignore
  demo tags, serialize publication, and retry missing or draft releases safely

- Restarting the only managed process no longer tries to split a removed tmux window
- Exited processes are replaced on start instead of being treated as healthy
- Worktree discovery now preserves spaces and newlines in valid paths
- Process names and shell paths inferred from project files are safely quoted
- Empty custom-detector or Python results can fall through to later detectors

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
