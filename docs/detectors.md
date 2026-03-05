# Custom Detectors

`wts init` auto-detects project types and generates `.wts.yaml` with inferred
processes. You can extend this with custom detector files.

## Built-in Detectors

| Detector   | Trigger file(s)      | What it infers |
|------------|----------------------|----------------|
| **nodejs** | `package.json`       | npm scripts; auto-picks pnpm/yarn/bun/npm based on lockfile |
| **go**     | `go.mod`             | `cmd/` sub-directories, or `go run .` fallback |
| **python** | `pyproject.toml`, `requirements.txt`, `setup.py` | Django (`manage.py`) commands, or generic poetry/uv/python runners |
| **makefile** | `Makefile`         | Makefile targets (skips `all`, `clean`, `install`, etc.) |

Built-ins run in the order listed above. The first match wins.

## Custom Detector Files

Drop YAML files into `~/.config/wts/detectors/` (or `$XDG_CONFIG_HOME/wts/detectors/`).
Each file defines a single detector.

### Format

```yaml
name: rust
description: Rust projects with Cargo
match:
  files:
    - Cargo.toml
processes:
  - name: run
    command: cargo run
  - name: test
    command: cargo watch -x test
  - name: check
    command: cargo watch -x check
```

### Fields

| Field                | Required | Description |
|----------------------|----------|-------------|
| `name`               | yes      | Unique identifier for this detector |
| `description`        | no       | Human-readable description |
| `match.files`        | yes      | List of files that must **all** exist for this detector to match |
| `processes`          | yes      | List of process entries to generate |
| `processes[].name`   | yes      | Process name (alphanumeric, `.`, `_`, `-`) |
| `processes[].command` | yes     | Shell command to run |

### Rules

- All files listed in `match.files` must exist in the project directory for the
  detector to trigger.
- Custom detectors run **after** all built-in detectors, so a built-in match
  takes priority.
- Invalid YAML files or files missing required fields are silently skipped.
- File extension must be `.yaml` or `.yml`.

### Examples

**Ruby on Rails:**

```yaml
name: rails
description: Rails projects
match:
  files:
    - Gemfile
    - config/routes.rb
processes:
  - name: server
    command: bundle exec rails server
  - name: console
    command: bundle exec rails console
  - name: test
    command: bundle exec rspec
```

**Elixir/Phoenix:**

```yaml
name: phoenix
description: Elixir Phoenix projects
match:
  files:
    - mix.exs
    - config/config.exs
processes:
  - name: server
    command: mix phx.server
  - name: test
    command: mix test --trace
  - name: iex
    command: iex -S mix
```

**Docker Compose:**

```yaml
name: docker-compose
description: Docker Compose projects
match:
  files:
    - docker-compose.yml
processes:
  - name: up
    command: docker compose up
  - name: build
    command: docker compose build
  - name: logs
    command: docker compose logs -f
```
