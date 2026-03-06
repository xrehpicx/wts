## wts pick

Pick a git worktree and switch

### Synopsis

Open an interactive selector (fzf if installed, fallback prompt otherwise) and switch to the chosen worktree.

```
wts pick [flags]
```

### Examples

```
wts pick
  wts pick --process demo-script
  wts pick --group dev
  wts pick --attach
```

### Options

```
      --attach           attach/focus tmux after switching
      --group string     process group name from config
  -h, --help             help for pick
      --process string   process profile name (default: first process in config)
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

