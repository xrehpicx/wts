## wts stop

Stop worktree process(es)

### Synopsis

Stop process windows managed by wts.

With no arguments it stops the active worktree process.
With a selector it stops only that worktree.
With --all it stops all discovered worktree windows.

```
wts stop [worktree] [flags]
```

### Examples

```
wts stop
  wts stop repo-main
  wts stop /abs/path/to/worktree
  wts stop --all
```

### Options

```
      --all    stop all discovered worktrees
  -h, --help   help for stop
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

