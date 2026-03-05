## wts

workswitch (wts: worktree switch) for moving processes across worktrees

### Synopsis

Process config lives in .wts.yaml.
Worktree directory assignments live in ~/.workswitch/state.yaml.

Use this when AI agents/devs work in multiple git worktrees and you want
fast process handoff with per-group preemption.

### Options

```
      --config string   path to .wts.yaml
  -h, --help            help for wts
      --state string    path to state file (default ~/.workswitch/state.yaml)
```

### SEE ALSO

* [wts add](wts_add.md)	 - Add a worktree directory assignment
* [wts assign](wts_assign.md)	 - Assign a process profile to a worktree
* [wts group](wts_group.md)	 - Set or clear group override for a worktree
* [wts list](wts_list.md)	 - List configured worktrees for this repo
* [wts logs](wts_logs.md)	 - Show recent tmux pane output for a worktree
* [wts next](wts_next.md)	 - Switch to next worktree in list
* [wts pick](wts_pick.md)	 - Pick a worktree and switch
* [wts prev](wts_prev.md)	 - Switch to previous worktree in list
* [wts processes](wts_processes.md)	 - List process profiles from config
* [wts remove](wts_remove.md)	 - Remove a worktree assignment
* [wts restart](wts_restart.md)	 - Restart process for a worktree
* [wts start](wts_start.md)	 - Start process for a worktree
* [wts status](wts_status.md)	 - Show worktree runtime status
* [wts stop](wts_stop.md)	 - Stop worktree process(es)
* [wts switch](wts_switch.md)	 - Switch to a worktree
* [wts tui](wts_tui.md)	 - Open interactive Bubble Tea TUI
* [wts validate](wts_validate.md)	 - Validate .wts.yaml
* [wts version](wts_version.md)	 - Show wts version

