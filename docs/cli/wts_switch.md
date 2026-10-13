## wts switch

Switch to a worktree and hand off a target

### Synopsis

Start or move a process profile or process group to a target worktree.

'switch' preempts: stops the previously active worktree, then starts the target.
'start' is additive: starts the target alongside any already running processes.
'restart' stops and re-starts the selected process or group.

'start' and 'restart' accept several worktrees to run the same target in each of
them; 'start --all' uses every available worktree. Every worktree is resolved
before anything starts. Processes receive WTS_WORKTREE, WTS_WORKTREE_DIR,
WTS_BRANCH, and WTS_PROCESS, so commands can choose distinct ports.

Process groups are configured in .wts.yaml and each member process still runs in
its own tmux pane.

```
wts switch <worktree> [flags]
```

### Examples

```
wts switch repo-main
  wts switch ../repo-agent --process demo-script
  wts switch ../repo-agent --group dev
  wts switch /abs/path/to/worktree --attach
```

### Options

```
      --attach           attach/focus tmux after command (single worktree only)
      --group string     process group name from config
  -h, --help             help for switch
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees
