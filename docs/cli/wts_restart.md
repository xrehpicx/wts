## wts restart

Restart a target in a worktree

### Synopsis

Start or move a process profile or process group to a target worktree.

'switch' preempts: stops the previously active worktree, then starts the target.
'start' is additive: starts the target alongside any already running processes.
'restart' stops and re-starts the selected process or group.

Process groups are configured in .wts.yaml and each member process still runs in
its own tmux pane.

```
wts restart <worktree> [flags]
```

### Examples

```
wts restart repo-main
  wts restart ../repo-agent --process demo-script
  wts restart ../repo-agent --group dev
  wts restart /abs/path/to/worktree --attach
```

### Options

```
      --attach           attach/focus tmux after command
      --group string     process group name from config
  -h, --help             help for restart
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

