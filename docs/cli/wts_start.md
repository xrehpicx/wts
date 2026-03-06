## wts start

Start a target in a worktree (additive)

### Synopsis

Start or move a process profile or process group to a target worktree.

'switch' preempts: stops the previously active worktree, then starts the target.
'start' is additive: starts the target alongside any already running processes.
'restart' stops and re-starts the selected process or group.

Process groups are configured in .wts.yaml and each member process still runs in
its own tmux pane.

```
wts start <worktree> [flags]
```

### Examples

```
wts start repo-main
  wts start ../repo-agent --process demo-script
  wts start ../repo-agent --group dev
  wts start /abs/path/to/worktree --attach
```

### Options

```
      --attach           attach/focus tmux after command
      --group string     process group name from config
  -h, --help             help for start
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

