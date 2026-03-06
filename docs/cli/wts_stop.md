## wts stop

Stop worktree process(es)

### Synopsis

Stop process windows managed by wts.

With no arguments it stops the active worktree process.
With a selector it stops only that worktree.
With --process it stops a specific process in the worktree.
With --group it stops all processes from that configured group in the worktree.
With --all it stops all discovered worktree windows.

```
wts stop [worktree] [flags]
```

### Examples

```
wts stop
  wts stop repo-main
  wts stop repo-main --process api
  wts stop repo-main --group dev
  wts stop /abs/path/to/worktree
  wts stop --all
```

### Options

```
      --all              stop all discovered worktrees
      --group string     stop all processes in a configured group (requires worktree argument)
  -h, --help             help for stop
      --process string   stop a specific process (requires worktree argument)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

