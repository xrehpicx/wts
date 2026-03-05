# workswitch

`workswitch` is a tmux-backed process handoff tool for Git worktrees.

- Binary: `wts`
- Aliases: `workswitch`, `wks`
- `wts` means `worktree switch`

## What it does

`wts` discovers worktrees directly from Git (`git worktree list --porcelain`) and lets you move a configured process between them.

When you switch/start/restart on a target worktree:

1. the previously active worktree process is stopped
2. the selected process profile starts in the target worktree dir

No extra `~/.workswitch/state.yaml` file is used.

## Requirements

- Go `1.22+`
- `tmux`
- `git`
- `make` (optional, for local workflow)

## Install

```bash
go install github.com/xrehpicx/wts@latest
```

## Quick Start

```bash
cd my-project
wts init          # auto-detects project type, generates .wts.yaml
wts tui           # open interactive TUI
```

## `wts init`

`wts init` inspects the current directory and generates a `.wts.yaml` with
inferred processes.

```bash
wts init                # detect & write .wts.yaml
wts init --dry-run      # preview without writing
wts init --force        # overwrite existing config
wts init --dir ../other # target a different directory
```

Built-in detectors: **Node.js** (package.json scripts), **Go** (cmd/ dirs),
**Python** (Django / pyproject.toml), **Makefile** (targets).

Custom detectors can be added as YAML files in `~/.config/wts/detectors/`.
See [docs/detectors.md](docs/detectors.md) for the format and examples.

## Config (`.wts.yaml`)

Only process profiles live in config.

```yaml
version: 1
defaults:
  stop_timeout_sec: 8
  shell: /bin/sh
processes:
  - name: api
    command: "go run ./cmd/api"
  - name: web
    command: "pnpm dev"
  - name: demo-script
    command: "./scripts/example-longrun.sh demo-script 3"
```

### Included long-running demo process

- [scripts/example-longrun.sh](scripts/example-longrun.sh)
- configured as `demo-script` in [.wts.example.yaml](.wts.example.yaml)

## Core commands

```bash
wts validate
wts processes
wts list
wts status
```

```bash
wts switch <worktree> --process api
wts next --process web
wts prev
wts restart <worktree> --process demo-script
wts stop                # stop active
wts stop <worktree>     # stop one
wts stop --all          # stop all discovered worktrees
wts logs <worktree> --lines 200
```

Selectors accept either worktree name or full path.

## TUI

```bash
wts tui
```

Shortcuts:

- `n` / `↓`: next worktree
- `p` / `↑`: previous worktree
- `←` / `[`: previous process profile
- `→` / `]`: next process profile
- `/`: search/filter processes by name
- `s` / `enter`: switch selected worktree
- `r`: restart selected worktree
- `x`: stop selected worktree
- `?`: toggle expanded help
- `q`: quit TUI

Exiting TUI does not stop the running process. It keeps running in tmux until you switch/stop it.

## Help and man pages

```bash
wts --help
wts switch --help
make docs
man ./docs/man/wts.1
```

## Local development

```bash
make help
make airflow
make run ARGS="tui"
make install
```

## Project docs

- [docs/detectors.md](docs/detectors.md) — custom detector plugin format
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md)
- [CHANGELOG.md](CHANGELOG.md)
- [LICENSE](LICENSE)
