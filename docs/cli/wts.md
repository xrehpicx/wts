## wts

workswitch (wts: worktree switch) process handoff for git worktrees

### Synopsis

Process config lives in .wts.yaml.
Worktrees are discovered live from: git worktree list --porcelain.

Switching preempts the previously active worktree process and starts the selected
process or group in the target worktree.

### Examples

```
wts validate
  wts list
  wts switch repo-main --process api
  wts switch repo-main --group dev
  wts next --process demo-script
  wts status --json
  wts tui
```

### Options

```
      --config string   path to .wts.yaml
  -h, --help            help for wts
```

### SEE ALSO

* [wts init](wts_init.md)	 - Generate .wts.yaml by detecting project type
* [wts list](wts_list.md)	 - List git worktrees for this repo
* [wts logs](wts_logs.md)	 - Show recent tmux pane output for a worktree
* [wts next](wts_next.md)	 - Switch to next git worktree in list
* [wts pick](wts_pick.md)	 - Pick a git worktree and switch
* [wts prev](wts_prev.md)	 - Switch to previous git worktree in list
* [wts processes](wts_processes.md)	 - List process profiles and groups from config
* [wts restart](wts_restart.md)	 - Restart a target in a worktree
* [wts start](wts_start.md)	 - Start a target in a worktree (additive)
* [wts status](wts_status.md)	 - Show worktree runtime status
* [wts stop](wts_stop.md)	 - Stop worktree process(es)
* [wts switch](wts_switch.md)	 - Switch to a worktree and hand off a target
* [wts tui](wts_tui.md)	 - Open interactive Bubble Tea TUI
* [wts validate](wts_validate.md)	 - Validate .wts.yaml
* [wts version](wts_version.md)	 - Show wts version

