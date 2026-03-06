## wts prev

Switch to previous git worktree in list

### Synopsis

Move to the previous discovered worktree and hand off the selected process profile or group.

```
wts prev [flags]
```

### Examples

```
wts prev
  wts prev --process api
  wts prev --group dev
  wts prev --attach
```

### Options

```
      --attach           attach/focus tmux after switching
      --group string     process group name from config
  -h, --help             help for prev
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

