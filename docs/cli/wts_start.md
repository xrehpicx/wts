## wts start

Start process in a worktree (preempts active)

### Synopsis

Start or move a process profile to a target worktree.

The currently active worktree process is stopped first, then the target process
is started in the selected worktree directory.

```
wts start <worktree> [flags]
```

### Examples

```
wts start repo-main
  wts start ../repo-agent --process demo-script
  wts start /abs/path/to/worktree --attach
```

### Options

```
      --attach           attach/focus tmux after command
  -h, --help             help for start
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

