## wts

workswitch (wts: worktree switch) for grouped dev processes via tmux

### Synopsis

workswitch manages one command per workspace directory and uses tmux
as the runtime source of truth.

Use groups to define preemption scope:
- switch/start within the same group stops the previously active workspace
- workspaces in different groups can run concurrently

Configuration discovery order:
1. .wts.yaml
2. .worktreeswitch.yaml (legacy)
3. .workswitch.yaml (legacy)

### Examples

```
  wts validate
  wts list
  wts switch api-main
  wts switch api-agent-a
  wts status --json
```

### Options

```
      --config string   path to .wts.yaml (legacy: .worktreeswitch.yaml/.workswitch.yaml)
  -h, --help            help for wts
```

### SEE ALSO

* [wts list](wts_list.md)	 - List configured workspaces
* [wts logs](wts_logs.md)	 - Show recent tmux pane output for a workspace
* [wts pick](wts_pick.md)	 - Interactively pick and switch to a workspace
* [wts restart](wts_restart.md)	 - Restart workspace process and make it active for its group
* [wts start](wts_start.md)	 - Start workspace process and make it active for its group
* [wts status](wts_status.md)	 - Show workspace runtime status
* [wts stop](wts_stop.md)	 - Stop workspace process(es)
* [wts switch](wts_switch.md)	 - Switch active workspace within its group
* [wts validate](wts_validate.md)	 - Validate .wts.yaml
* [wts version](wts_version.md)	 - Show wts version

