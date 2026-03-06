## wts logs

Show recent tmux pane output for a worktree

### Synopsis

Capture recent output lines from the tmux window for a running worktree process.

```
wts logs <worktree> [flags]
```

### Examples

```
wts logs repo-main
  wts logs repo-main --process api
  wts logs ../repo-agent --lines 400
```

### Options

```
  -h, --help             help for logs
      --lines int        number of log lines (default 200)
      --process string   process name (default: first pane)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

