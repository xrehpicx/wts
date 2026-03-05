# workswitch

`workswitch` is a tmux-backed process switcher for multi-worktree development.

Binary: `wts` (short for **worktree switch**).

## Model

`workswitch` now uses a strict split:

1. `.wts.yaml` (repo local): process profiles only
2. `~/.workswitch/state.yaml` (user global): per-repo worktree directories and assignments

This is optimized for AI-agent workflows where multiple git worktrees exist and you want to move long-running dev processes between those directories quickly.

## Requirements

- Go 1.22+
- `tmux`
- `make`

## Install

```bash
go install github.com/xrehpicx/wts@latest
```

## Process config (`.wts.yaml`)

```yaml
version: 1
defaults:
  stop_timeout_sec: 8
  shell: /bin/sh
processes:
  - name: api
    command: "go run ./cmd/api"
    group: backend
  - name: web
    command: "pnpm dev"
    group: frontend
  - name: demo-script
    command: "./scripts/example-longrun.sh demo-script 3"
    group: demo
```

Notes:

- no worktree directories are stored in `.wts.yaml`
- directory assignments are managed with CLI/TUI and saved in `~/.workswitch/state.yaml`

## Long-running example script

- [scripts/example-longrun.sh](scripts/example-longrun.sh) is included as a reusable demo process.
- It is referenced in [.wts.example.yaml](.wts.example.yaml) as `demo-script`.

## Worktree management

Add worktrees for current repo:

```bash
wts add ../repo-main --name main --process api
wts add ../repo-agent-a --name agent-a --process api
wts add ../repo-web --name web-main --process web
```

View assignments:

```bash
wts list
wts processes
```

Adjust assignment/group:

```bash
wts assign agent-a --process web
wts group agent-a --set backend
wts group agent-a --clear
wts remove web-main
```

## Runtime commands

```bash
wts switch main
wts next
wts prev
wts restart agent-a
wts status
wts logs main --lines 200
wts stop --group backend
wts stop --all
```

## TUI (Bubble Tea)

Launch interactive UI:

```bash
wts tui
```

TUI shortcuts:

- `n` / `p` (or arrows): move next/prev worktree
- `s` / `enter`: switch to selected worktree
- `r`: restart selected
- `x`: stop selected
- `a`: add current repo root as a worktree entry
- `d`: remove selected worktree entry
- `[` / `]`: cycle process profile on selected worktree
- `g`: set selected worktree group override to process group
- `u`: clear group override
- `q`: quit

## Help and docs

```bash
wts --help
wts switch --help
make docs
man ./docs/man/wts.1
```

## Make targets

```bash
make help
```

Primary flow:

1. `make airflow` runs `check` + `build`
2. `make check` runs `tidy`, `fmt`, `vet`, `lint`, `test`
3. `make coverage` writes `coverage.out`
4. `make install` installs `wts` to `GOPATH/bin`

## Project docs

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md)
- [CHANGELOG.md](CHANGELOG.md)
- [LICENSE](LICENSE)
