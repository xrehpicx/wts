## wts status

Show worktree runtime status

### Synopsis

Show running/active status for discovered worktrees and the currently associated process profile.

```
wts status [worktree] [flags]
```

### Examples

```
wts status
  wts status repo-main
  wts status --json
```

### Options

```
  -h, --help   help for status
      --json   output JSON
```

### Options inherited from parent commands

```
      --config string   path to .wts.yaml
```

### SEE ALSO

* [wts](wts.md)	 - workswitch (wts: worktree switch) process handoff for git worktrees

