## wts tui

Open interactive Bubble Tea TUI

### Synopsis

Open the interactive TUI for worktree/process handoff.

The TUI lets you move selection across discovered worktrees, choose process
profiles, and switch/restart/stop quickly.

Shortcuts:
  n/down   next worktree
  p/up     previous worktree
  [ / ]    previous/next process profile
  s/enter  switch
  r        restart
  x        stop
  ?        toggle shortcut help
  q        quit

Exiting TUI does not stop running worktree processes.

```
wts tui [flags]
```

### Examples

```
wts tui
```

### Options

```
  -h, --help   help for tui
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

