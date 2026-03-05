## wts list

List configured workspaces

### Synopsis

List all configured workspaces from the resolved config file.

Shows workspace name, effective group, absolute directory, and configured command.

```
wts list [flags]
```

### Examples

```
  wts list
  wts --config ./my-config.yaml list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) for grouped dev processes via tmux

