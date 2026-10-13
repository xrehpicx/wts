# workswitch

`wts` is a tmux-backed CLI for moving dev servers and other long-running processes between Git worktrees.

It discovers worktrees from Git, reads process targets from `.wts.yaml`, and lets you hand off a single process or a process group to the worktree you want to work in.

https://github.com/user-attachments/assets/5705d308-a176-412f-b80f-af519fdf76f1

The installed command is `wts`.

## Requirements

- Go `1.26+` (for installation from source)
- `git`
- `tmux`

## Install

Download a prebuilt binary from [Releases](https://github.com/xrehpicx/wts/releases/latest),
or install from source:

```bash
go install github.com/xrehpicx/wts@latest
```

## Update

```bash
go install github.com/xrehpicx/wts@latest
```

## Quick Start

```bash
cd my-project
wts init
wts
```

`wts` opens the worktree/process switcher. `wts init` generates `.wts.yaml` for the current repo. `wts tui` remains available as an explicit alias for the TUI.

Use `make airflow` before submitting changes. It checks formatting, dependencies,
static analysis, race-tested coverage, known vulnerabilities, and the production
build—the same verification performed on every GitHub push and pull request.
This development command requires GNU Make and a POSIX-compatible shell; on
Windows, `go test -shuffle=on ./...` runs the platform test suite directly.

## Running a target in several worktrees

`switch` hands a process or group to one worktree and stops the previously active
one. To run the same process or group side by side, use `start`, which is
additive and accepts several worktrees:

```bash
wts start repo-main repo-feature --group dev
wts start --all --process api       # every available worktree
wts restart repo-main repo-feature --group dev
wts stop repo-main repo-feature --group dev
```

Every worktree is resolved before anything starts, so a misspelled name changes
nothing. In the TUI, mark worktrees with `space` and press `enter`.

Each process receives `WTS_WORKTREE`, `WTS_WORKTREE_DIR`, `WTS_BRANCH`, and
`WTS_PROCESS`, so copies running at the same time can avoid conflicts, for
example by choosing a port per worktree:

```yaml
processes:
  - name: web
    # A stable port per worktree, between 3000 and 3999
    command: PORT=$((3000 + $(printf %s "$WTS_WORKTREE" | cksum | cut -d' ' -f1) % 1000)) pnpm dev
```

Values set under `env` take precedence over these variables.

## TUI controls

The header shows the selected worktree and target, so the destination of each
command is clear. Wide terminals show worktrees and output side by side; narrow
terminals use a compact worktree list above the output.

| Key | Action |
| --- | --- |
| `j` / `k`, `↑` / `↓` | Select a worktree |
| `h` / `l`, `←` / `→` | Select a process or group |
| `enter` / `s` | Start the target, or switch it to the selected worktree |
| `r` / `x` | Restart / stop the selected target |
| `X` | Stop all processes in the selected worktree |

While worktrees are marked, `enter`, `r`, `x`, and `X` act on every marked
worktree instead of the selected one, and never stop other worktrees.
| `a` | Attach to the selected target in tmux |
| `space` | Mark a worktree; `esc` clears marks |
| `/` | Search targets; use `↑` / `↓` to choose a match |
| `g` | Create a group; `tab` changes focus, `space` selects members, `enter` saves |
| `esc` | Cancel search or group creation |
| `?` | Show all shortcuts |
| `q` / `ctrl+c` | Quit; running processes stay in tmux |

Group output shows each process's recent lines with its name; it does not infer
chronological ordering between independent processes. Errors appear on a separate
header line, and target changes fetch output immediately.

## Development and releases

`make airflow` enforces at least 60% total test coverage. `make test-e2e` runs the
CLI against real Git worktrees and an isolated tmux server. CLI reference pages
and man pages are regenerated with `make docs`.

After committing and pushing verified changes to `main`, run:

```bash
make release BUMP=minor  # omit BUMP for a patch release
```

This dispatches the GitHub release workflow for the exact current commit. The
workflow checks it again, creates a semantic version tag, and publishes archives
and checksums. Follow it with `gh run list --workflow release.yml`. Repeating the
command without new commits retries missing or draft releases and leaves a
published release unchanged.

## More

- [docs/detectors.md](docs/detectors.md)
- [CHANGELOG.md](CHANGELOG.md)
