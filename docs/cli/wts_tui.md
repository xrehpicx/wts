## wts tui

Open interactive Bubble Tea TUI

### Synopsis

Open the interactive TUI for worktree/process handoff.

The TUI lets you move selection across discovered worktrees, choose process
profiles or groups, and start/restart/stop quickly. Multiple processes can run
simultaneously in the same worktree as separate tmux panes, including every
member of a configured group. Groups are defined in .wts.yaml and appear in the
target selector as [group] <name>. Press g to create a group and save it back
to the current repo's .wts.yaml.

Search lists matching targets: use up/down to choose, enter to select, or esc to
cancel. The group editor uses tab to change focus, space to toggle members, enter
to save, and esc to cancel. Ctrl+c quits from any screen. Narrow terminals show a
compact worktree list; errors stay visible on a separate header line.

Shortcuts:
  j/↓      next worktree        h/←    prev target
  k/↑      prev worktree        l/→    next target
  s/enter  start/switch target   a      attach tmux
  r        restart target
  x        stop selected target  /      search target by name
  g        create group in .wts.yaml
  X        stop all in worktree  ?      toggle full help
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
