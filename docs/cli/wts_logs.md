## wts logs

Show recent tmux pane output for a workspace

### Synopsis

Capture output from the workspace's tmux pane history.

Use --lines to choose how many historical lines to print.

```
wts logs <workspace> [flags]
```

### Examples

```
  wts logs api-main
  wts logs api-main --lines 500
```

### Options

```
  -h, --help        help for logs
      --lines int   number of log lines to capture (default 200)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

