## wts switch

Switch active workspace within its group

### Synopsis

Activate a workspace with group preemption semantics.

Behavior:
- if another workspace is active in the same group, it is stopped first
- if target workspace is already running, it is reused
- marks the target as active for its group

```
wts switch <workspace> [flags]
```

### Examples

```
  wts switch api-main
  wts switch api-agent-b --attach
```

### Options

```
      --attach   attach/focus tmux after switching
  -h, --help     help for switch
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

