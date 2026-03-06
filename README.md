# workswitch

`workswitch` is a tmux-backed process handoff tool for Git worktrees.

- Binary: `wts`
- Aliases: `workswitch`, `wks`
- `wts` means `worktree switch`

## What it does

`wts` discovers worktrees directly from Git (`git worktree list --porcelain`) and lets you move a configured process, or a configured process group, between them.

When you switch/start/restart on a target worktree:

1. the previously active worktree process is stopped
2. the selected process profile, or every member of the selected process group, starts in the target worktree dir

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

To install a specific version:

```bash
go install github.com/xrehpicx/wts@v0.2.0
```

To update to the latest release, re-run the install command above.

Verify:

```bash
wts version
# wts 0.2.0 (03e4f59)
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

Process profiles and optional process groups live in config.

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
groups:
  - name: dev
    processes:
      - api
      - web
```

Groups are launch targets, not merged commands. Each member still runs in its own tmux pane.

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
wts switch <worktree> --group dev
wts next --process web
wts next --group dev
wts prev
wts restart <worktree> --process demo-script
wts restart <worktree> --group dev
wts stop                # stop active
wts stop <worktree>     # stop one
wts stop <worktree> --group dev
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
- `←` / `[`: previous process/group target
- `→` / `]`: next process/group target
- `/`: search/filter process and group targets by name
- `s` / `enter`: switch/start selected target in the selected worktree
- `r`: restart selected target
- `x`: stop selected target
- `g`: create a new group and save it into this repo’s `.wts.yaml`
- `?`: toggle expanded help
- `q`: quit TUI

Exiting TUI does not stop the running process. It keeps running in tmux until you switch/stop it.

Groups appear in the TUI as `[group] <name>` entries. Press `g` to open the in-TUI group editor, choose member processes, and save the new group back into that repo’s `.wts.yaml`.

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

## Versioning

This project follows [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH`).

| Bump    | When                                                                 | Example                                    |
|---------|----------------------------------------------------------------------|--------------------------------------------|
| `PATCH` | Bug fixes, lint fixes, docs — no behavior change                     | `v0.2.0` → `v0.2.1`                       |
| `MINOR` | New features, new commands, UX improvements — backwards compatible   | `v0.2.1` → `v0.3.0`                       |
| `MAJOR` | Breaking changes to CLI flags, config format, or removed commands    | `v0.3.0` → `v1.0.0`                       |

### Releasing a new version

1. Update `version` in `main.go`
2. Add a section to `CHANGELOG.md`
3. Commit, tag, and push:

```bash
git tag v0.2.0
git push origin v0.2.0
```

Users update with:

```bash
go install github.com/xrehpicx/wts@latest
```

## Project docs

- [docs/detectors.md](docs/detectors.md) — custom detector plugin format
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md)
- [CHANGELOG.md](CHANGELOG.md)
- [LICENSE](LICENSE)
