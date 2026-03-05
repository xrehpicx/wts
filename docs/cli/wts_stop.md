## wts stop

Stop workspace process(es)

### Synopsis

Stop one or more workspace processes.

You must choose exactly one scope:
- workspace argument: stop one workspace
- --group <group>: stop current active workspace in that group
- --all: stop all configured workspaces

```
wts stop [workspace] [flags]
```

### Examples

```
  wts stop api-main
  wts stop --group backend
  wts stop --all
```

### Options

```
      --all            stop all running workspace processes
      --group string   stop active workspace in group
  -h, --help           help for stop
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

