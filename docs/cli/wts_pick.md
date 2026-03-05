## wts pick

Interactively pick and switch to a workspace

### Synopsis

Interactively choose a workspace and switch to it.

Picker behavior:
- uses fzf when available
- falls back to built-in numbered prompt when fzf is missing

```
wts pick [flags]
```

### Examples

```
  wts pick
  wts pick --attach
```

### Options

```
      --attach   attach/focus tmux after switching
  -h, --help     help for pick
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

