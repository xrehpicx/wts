## wts next

Switch to next git worktree in list

### Synopsis

Move to the next discovered worktree and hand off the selected process profile or group.

```
wts next [flags]
```

### Examples

```
wts next
  wts next --process demo-script
  wts next --group dev
  wts next --attach
```

### Options

```
      --attach           attach/focus tmux after switching
      --group string     process group name from config
  -h, --help             help for next
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

