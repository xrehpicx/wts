# wks

`wks` is a tmux-backed CLI that manages one process per workspace directory.
It enforces one active process per `group`, so switching within a group preempts the previous workspace.

## Requirements

- Go 1.22+
- `tmux`
- `make`

## Install

```bash
go install github.com/xrehpicx/wks@latest
```

## Quickstart

1. Create `.workswitch.yaml` in your repo root:

```yaml
version: 1
defaults:
  stop_timeout_sec: 8
  shell: /bin/sh
workspaces:
  - name: api
    dir: ./services/api
    command: "go run ./cmd/api"
    group: backend
  - name: worker
    dir: ./services/worker
    command: "go run ./cmd/worker"
    group: backend
  - name: web
    dir: ./apps/web
    command: "pnpm dev"
    group: frontend
```

2. Validate and run:

```bash
make airflow
./bin/wks validate
./bin/wks list
./bin/wks switch api
./bin/wks switch worker   # stops api (same group)
./bin/wks switch web      # does not stop worker (different group)
```

## Core commands

- `wks list`
- `wks switch <workspace> [--attach]`
- `wks start <workspace> [--attach]`
- `wks restart <workspace> [--attach]`
- `wks stop <workspace|--group <name>|--all>`
- `wks status [workspace] [--json]`
- `wks logs <workspace> [--lines 200]`
- `wks pick [--attach]`
- `wks validate`
- `wks version`

## Make targets

```bash
make help
```

Primary flow:

1. `make airflow` runs `check` + `build`
2. `make check` runs `tidy`, `fmt`, `vet`, `lint`, `test`
3. `make coverage` writes `coverage.out`
4. `make install` installs binary to `GOPATH/bin`

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
