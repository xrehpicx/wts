## wts status

Show workspace runtime status

### Synopsis

Show current runtime status from tmux metadata and windows.

Fields:
- running: tmux window for workspace exists
- active: workspace matches active marker for its group

```
wts status [workspace] [flags]
```

### Examples

```
  wts status
  wts status api-main
  wts status --json
```

### Options

```
  -h, --help   help for status
      --json   output JSON
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

