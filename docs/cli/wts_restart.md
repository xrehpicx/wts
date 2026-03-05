## wts restart

Restart workspace process and make it active for its group

### Synopsis

Force restart the target workspace process.

This always stops and starts the target workspace, then marks it active
for its group.

```
wts restart <workspace> [flags]
```

### Examples

```
  wts restart api-main
  wts restart api-main --attach
```

### Options

```
      --attach   attach/focus tmux after restarting
  -h, --help     help for restart
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

