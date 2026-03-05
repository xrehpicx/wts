## wts init

Generate .wts.yaml by detecting project type

### Synopsis

Inspect the current (or specified) directory, detect the project type, and
generate a .wts.yaml with inferred processes.

Built-in detectors:
  nodejs     package.json scripts (auto-detects npm/pnpm/yarn/bun)
  go         cmd/ sub-directories or go run .
  python     manage.py (Django) or pyproject.toml / requirements.txt
  makefile   Makefile targets

Custom detectors can be added as YAML files in:
  ~/.config/wts/detectors/

See 'wts init --help' or docs/detectors.md for the file format.

```
wts init [flags]
```

### Examples

```
wts init
  wts init --dir ../my-project
  wts init --force
  wts init --dry-run
```

### Options

```
      --dir string   project directory (default: current working directory)
      --dry-run      print generated config without writing
      --force        overwrite existing .wts.yaml
  -h, --help         help for init
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

