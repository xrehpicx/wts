## wts validate

Validate .wts.yaml

### Synopsis

Load and validate configuration file schema and workspace directories.

Checks include:
- config version
- workspace uniqueness
- command presence
- workspace directory existence

```
wts validate [flags]
```

### Examples

```
  wts validate
  wts --config ./tmp/dev.wts.yaml validate
```

### Options

```
  -h, --help   help for validate
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

