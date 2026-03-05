## wts start

Start workspace process and make it active for its group

### Synopsis

Start a workspace process using the configured command in its directory.

Like switch, start preempts the currently active workspace in the same group.
If target is already running, it is reused unless you run restart.

```
wts start <workspace> [flags]
```

### Examples

```
  wts start web-main
  wts start web-agent-a --attach
```

### Options

```
      --attach   attach/focus tmux after starting
  -h, --help     help for start
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

