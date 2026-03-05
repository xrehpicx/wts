# workswitch

`workswitch` is a tmux-backed process switcher for development workspaces.
The command is `wts`, which stands for **worktree switch**.

It is designed for teams using AI agents + Git worktrees, where each worktree has the same app stack but only one process should be active within a group at a time.

## Why this exists

When AI agents work in parallel Git worktrees, developers often need to move a shared dev server between worktree directories quickly. `workswitch` lets you model that directly:

- define multiple workspaces (worktree directories)
- tie them with a group (for example `backend`)
- switching within that group preempts the previous workspace process

## Requirements

- Go 1.22+
- `tmux`
- `make`

## Install

```bash
go install github.com/xrehpicx/wts@latest
```

## Configuration

Default config file: `.wts.yaml`

Also supported for compatibility:

- `.worktreeswitch.yaml`
- `.workswitch.yaml`

Example config:

```yaml
version: 1
defaults:
  stop_timeout_sec: 8
  shell: /bin/sh
workspaces:
  - name: wt-main
    dir: ../repo-main
    command: "pnpm dev"
    group: web
  - name: wt-agent-a
    dir: ../repo-agent-a
    command: "pnpm dev"
    group: web
  - name: api-main
    dir: ../repo-main
    command: "go run ./cmd/api"
    group: backend
  - name: api-agent-b
    dir: ../repo-agent-b
    command: "go run ./cmd/api"
    group: backend
```

## Quickstart

```bash
make airflow
./bin/wts validate
./bin/wts list
./bin/wts switch api-main
./bin/wts switch api-agent-b   # stops api-main (same group)
./bin/wts switch wt-main       # independent if in different group
./bin/wts status
./bin/wts stop --all
```

## Help and command docs

CLI help (Cobra-powered):

```bash
wts --help
wts switch --help
wts stop --help
```

Generate and browse full command docs:

```bash
make docs
ls docs/cli
```

Man pages are generated into `docs/man`:

```bash
man ./docs/man/wts.1
man ./docs/man/wts-switch.1
```

## Core commands

- `wts list`
- `wts switch <workspace> [--attach]`
- `wts start <workspace> [--attach]`
- `wts restart <workspace> [--attach]`
- `wts stop <workspace|--group <name>|--all>`
- `wts status [workspace] [--json]`
- `wts logs <workspace> [--lines 200]`
- `wts pick [--attach]`
- `wts validate`
- `wts version`

## Make targets

```bash
make help
```

Primary flow:

1. `make airflow` runs `check` + `build`
2. `make check` runs `tidy`, `fmt`, `vet`, `lint`, `test`
3. `make coverage` writes `coverage.out`
4. `make install` installs `wts` to `GOPATH/bin`

## Runtime model

- One tmux session per repo.
- One tmux window per workspace (`ws:<workspace>`).
- Group activity is tracked in tmux session options.
- Tmux is the source of runtime truth (no separate state DB).

## Project docs

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md)
- [CHANGELOG.md](CHANGELOG.md)
- [LICENSE](LICENSE)
