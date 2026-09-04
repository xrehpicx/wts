package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/xrehpicx/wts/internal/config"
	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

type app struct {
	version    string
	commit     string
	configPath string
	in         io.Reader
	out        io.Writer
	err        io.Writer

	newBackend func() runtime.Backend
	runTUI     func(context.Context) error
}

type runtimeContext struct {
	ctx        context.Context
	project    *model.Project
	repoRoot   string
	worktrees  []gitwt.Worktree
	manager    *runtime.Manager
	newBackend func() runtime.Backend
}

func (rc *runtimeContext) context() context.Context {
	if rc.ctx != nil {
		return rc.ctx
	}
	return context.Background()
}

func NewRootCmd(version, commit string) *cobra.Command {
	a := &app{
		version: version,
		commit:  commit,
		in:      os.Stdin,
		out:     os.Stdout,
		err:     os.Stderr,
		newBackend: func() runtime.Backend {
			return tmux.NewClient("tmux")
		},
	}

	return a.newRootCmd()
}

func (a *app) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "wts",
		Short: "workswitch (wts: worktree switch) process handoff for git worktrees",
		Long: `Process config lives in .wts.yaml.
Worktrees are discovered live from: git worktree list --porcelain.

Switching preempts the previously active worktree process and starts the selected
process or group in the target worktree.`,
		Example: strings.TrimSpace(`
  wts
  wts validate
  wts list
  wts switch repo-main --process api
  wts switch repo-main --group dev
  wts next --process demo-script
  wts status --json
  wts tui
`),
		Args: cobra.NoArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			a.in, a.out, a.err = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runTUICommand(cmd.Context())
		},
	}
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "path to .wts.yaml")
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.err)

	root.AddCommand(a.newInitCmd())
	root.AddCommand(a.newValidateCmd())
	root.AddCommand(a.newProcessesCmd())
	root.AddCommand(a.newListCmd())
	root.AddCommand(a.newSwitchCmd())
	root.AddCommand(a.newStartCmd())
	root.AddCommand(a.newRestartCmd())
	root.AddCommand(a.newNextCmd())
	root.AddCommand(a.newPrevCmd())
	root.AddCommand(a.newStopCmd())
	root.AddCommand(a.newStatusCmd())
	root.AddCommand(a.newLogsCmd())
	root.AddCommand(a.newPickCmd())
	root.AddCommand(a.newTUICmd())
	root.AddCommand(a.newVersionCmd())

	return root
}

func Execute(version, commit string) error {
	return NewRootCmd(version, commit).Execute()
}

func (a *app) runTUICommand(ctx context.Context) error {
	if a.runTUI != nil {
		return a.runTUI(ctx)
	}
	return a.withRuntime(ctx, func(rc *runtimeContext) error {
		m := newTUIModel(rc)
		p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(a.in), tea.WithOutput(a.out))
		finalModel, err := p.Run()
		if err != nil {
			return err
		}
		if tm, ok := finalModel.(*tuiModel); ok {
			if tm.attachSpec != nil {
				return tm.rc.manager.Attach(ctx, *tm.attachSpec)
			}
			if tm.quitInfo != "" {
				_, err := fmt.Fprint(a.out, tm.quitInfo)
				return err
			}
		}
		return nil
	})
}

func (a *app) withProject(fn func(*model.Project) error) error {
	project, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	return fn(project)
}

func (a *app) withRuntime(ctx context.Context, fn func(*runtimeContext) error) error {
	return a.withProject(func(project *model.Project) error {
		repoRoot, err := resolveRepoRoot(ctx, project.RootDir)
		if err != nil {
			return err
		}
		worktrees, err := gitwt.DiscoverContext(ctx, repoRoot)
		if err != nil {
			return err
		}
		rc := &runtimeContext{
			ctx:        ctx,
			project:    project,
			repoRoot:   repoRoot,
			worktrees:  worktrees,
			manager:    runtime.NewManager(project, repoRoot, worktrees, a.newBackend()),
			newBackend: a.newBackend,
		}
		return fn(rc)
	})
}

func (a *app) newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .wts.yaml",
		Long:  "Validate the process/group configuration file and print loaded target counts.",
		Example: strings.TrimSpace(`
  wts validate
  wts validate --config ../other/.wts.yaml
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withProject(func(p *model.Project) error {
				_, err := fmt.Fprintf(a.out, "config valid: %s (%d processes, %d groups)\n", displayField(p.ConfigPath), len(p.Processes), len(p.Groups))
				return err
			})
		},
	}
}

func (a *app) newProcessesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "processes",
		Short: "List process profiles and groups from config",
		Long:  "Show process profile commands and configured process groups from .wts.yaml.",
		Example: strings.TrimSpace(`
  wts processes
  wts processes --config ../other/.wts.yaml
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withProject(func(p *model.Project) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "TYPE\tNAME\tDETAIL")
				for _, proc := range p.Processes {
					_, _ = fmt.Fprintf(tw, "process\t%s\t%s\n", displayField(proc.Name), displayField(proc.Command))
				}
				for _, group := range p.Groups {
					_, _ = fmt.Fprintf(tw, "group\t%s\t%s\n", displayField(group.Name), displayField(strings.Join(group.Processes, ", ")))
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List git worktrees for this repo",
		Long:  "Discover worktrees using Git and print worktree name, branch, and absolute directory.",
		Example: strings.TrimSpace(`
  wts list
`),
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKTREE\tBRANCH\tDIR")
				for _, wt := range rc.manager.ListWorktrees() {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", displayField(wt.Name), displayField(wt.Branch), displayField(wt.Dir))
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newSwitchCmd() *cobra.Command {
	return a.newRunCmd("switch", "Switch to a worktree and hand off a target", (*runtime.Manager).Switch)
}

func (a *app) newStartCmd() *cobra.Command {
	return a.newRunCmd("start", "Start a target in a worktree (additive)", (*runtime.Manager).Start)
}

func (a *app) newRestartCmd() *cobra.Command {
	return a.newRunCmd("restart", "Restart a target in a worktree", (*runtime.Manager).Restart)
}

func (a *app) newRunCmd(name, short string, fn func(*runtime.Manager, context.Context, string, runtime.RunOptions) error) *cobra.Command {
	var (
		attach  bool
		process string
		group   string
	)
	cmd := &cobra.Command{
		Use:   name + " <worktree>",
		Short: short,
		Long: strings.TrimSpace(`
Start or move a process profile or process group to a target worktree.

'switch' preempts: stops the previously active worktree, then starts the target.
'start' is additive: starts the target alongside any already running processes.
'restart' stops and re-starts the selected process or group.

Process groups are configured in .wts.yaml and each member process still runs in
its own tmux pane.`),
		Example: strings.TrimSpace(fmt.Sprintf(`
  wts %s repo-main
  wts %s ../repo-agent --process demo-script
  wts %s ../repo-agent --group dev
  wts %s /abs/path/to/worktree --attach
`, name, name, name, name)),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := runOptionsFromFlags(strings.TrimSpace(process), strings.TrimSpace(group), attach)
			if err != nil {
				return err
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if err := fn(rc.manager, cmd.Context(), args[0], opts); err != nil {
					return err
				}
				target, _ := rc.project.ResolveTarget(opts.Process, opts.Group)
				message := "✓ started %s in %s\n"
				switch name {
				case "switch":
					message = "✓ switched %s to %s\n"
				case "restart":
					message = "✓ restarted %s in %s\n"
				}
				_, err := fmt.Fprintf(a.out, message, displayField(formatTargetLabel(target)), displayField(args[0]))
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after command")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	cmd.Flags().StringVar(&group, "group", "", "process group name from config")
	return cmd
}

func (a *app) newNextCmd() *cobra.Command {
	var (
		attach  bool
		process string
		group   string
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Switch to next git worktree in list",
		Long:  "Move to the next discovered worktree and hand off the selected process profile or group.",
		Example: strings.TrimSpace(`
  wts next
  wts next --process demo-script
  wts next --group dev
  wts next --attach
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := runOptionsFromFlags(strings.TrimSpace(process), strings.TrimSpace(group), attach)
			if err != nil {
				return err
			}
			return a.cycleAndSwitch(cmd.Context(), 1, opts)
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	cmd.Flags().StringVar(&group, "group", "", "process group name from config")
	return cmd
}

func (a *app) newPrevCmd() *cobra.Command {
	var (
		attach  bool
		process string
		group   string
	)
	cmd := &cobra.Command{
		Use:   "prev",
		Short: "Switch to previous git worktree in list",
		Long:  "Move to the previous discovered worktree and hand off the selected process profile or group.",
		Example: strings.TrimSpace(`
  wts prev
  wts prev --process api
  wts prev --group dev
  wts prev --attach
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := runOptionsFromFlags(strings.TrimSpace(process), strings.TrimSpace(group), attach)
			if err != nil {
				return err
			}
			return a.cycleAndSwitch(cmd.Context(), -1, opts)
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	cmd.Flags().StringVar(&group, "group", "", "process group name from config")
	return cmd
}

func (a *app) cycleAndSwitch(ctx context.Context, delta int, opts runtime.RunOptions) error {
	return a.withRuntime(ctx, func(rc *runtimeContext) error {
		items := rc.manager.ListWorktrees()
		if len(items) == 0 {
			return fmt.Errorf("no git worktrees found (create one with: git worktree add ../branch-name)")
		}

		rows, err := rc.manager.Status(ctx, "")
		if err != nil {
			return err
		}
		activeDir := ""
		for _, row := range rows {
			if row.Active {
				activeDir = filepath.Clean(row.Dir)
				break
			}
		}
		next := nextWorktreeIndex(items, activeDir, delta)
		if err := rc.manager.Switch(ctx, items[next].Dir, opts); err != nil {
			return err
		}
		_, err = fmt.Fprintf(a.out, "✓ switched to %s\n", displayField(items[next].Name))
		return err
	})
}

func nextWorktreeIndex(items []gitwt.Worktree, activeDir string, delta int) int {
	if len(items) == 0 {
		return -1
	}
	activeIdx := -1
	if activeDir != "" {
		for i := range items {
			if filepath.Clean(items[i].Dir) == filepath.Clean(activeDir) {
				activeIdx = i
				break
			}
		}
	}
	if activeIdx == -1 {
		if delta < 0 {
			return len(items) - 1
		}
		return 0
	}
	return (activeIdx + delta%len(items) + len(items)) % len(items)
}

func (a *app) newStopCmd() *cobra.Command {
	var (
		all     bool
		process string
		group   string
	)
	cmd := &cobra.Command{
		Use:   "stop [worktree]",
		Short: "Stop worktree process(es)",
		Long: strings.TrimSpace(`
Stop process windows managed by wts.

With no arguments it stops the active worktree process.
With a selector it stops only that worktree.
With --process it stops a specific process in the worktree.
With --group it stops all processes from that configured group in the worktree.
With --all it stops all discovered worktree windows.`),
		Example: strings.TrimSpace(`
  wts stop
  wts stop repo-main
  wts stop repo-main --process api
  wts stop repo-main --group dev
  wts stop /abs/path/to/worktree
  wts stop --all
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proc := strings.TrimSpace(process)
			groupName := strings.TrimSpace(group)
			if err := validateStopSelection(all, proc, groupName, args); err != nil {
				return err
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				switch {
				case all:
					if err := rc.manager.StopAll(cmd.Context()); err != nil {
						return err
					}
					_, err := fmt.Fprintln(a.out, "✓ stopped all worktrees")
					return err
				case len(args) == 1 && groupName != "":
					if err := rc.manager.StopGroup(cmd.Context(), args[0], groupName); err != nil {
						return err
					}
					_, err := fmt.Fprintf(a.out, "✓ stopped group %s in %s\n", displayField(groupName), displayField(args[0]))
					return err
				case len(args) == 1 && proc != "":
					if err := rc.manager.StopProcess(cmd.Context(), args[0], proc); err != nil {
						return err
					}
					_, err := fmt.Fprintf(a.out, "✓ stopped %s in %s\n", displayField(proc), displayField(args[0]))
					return err
				case len(args) == 1:
					if err := rc.manager.StopWorktree(cmd.Context(), args[0]); err != nil {
						return err
					}
					_, err := fmt.Fprintf(a.out, "✓ stopped %s\n", displayField(args[0]))
					return err
				default:
					if err := rc.manager.StopActive(cmd.Context()); err != nil {
						return err
					}
					_, err := fmt.Fprintln(a.out, "✓ stopped active worktree")
					return err
				}
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "stop all discovered worktrees")
	cmd.Flags().StringVar(&process, "process", "", "stop a specific process (requires worktree argument)")
	cmd.Flags().StringVar(&group, "group", "", "stop all processes in a configured group (requires worktree argument)")
	return cmd
}

func validateStopSelection(all bool, process, group string, args []string) error {
	if process != "" && group != "" {
		return fmt.Errorf("--process and --group cannot be combined")
	}
	if all && len(args) > 0 {
		return fmt.Errorf("--all cannot be combined with a worktree selector")
	}
	if all && (process != "" || group != "") {
		return fmt.Errorf("--all cannot be combined with --process or --group")
	}
	if len(args) == 0 && (process != "" || group != "") {
		return fmt.Errorf("--process and --group require a worktree selector")
	}
	return nil
}

func (a *app) newStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status [worktree]",
		Short: "Show worktree runtime status",
		Long:  "Show running/active status for discovered worktrees and the currently associated process profile.",
		Example: strings.TrimSpace(`
  wts status
  wts status repo-main
  wts status --json
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			worktree := ""
			if len(args) == 1 {
				worktree = args[0]
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if jsonOut {
					payload, err := rc.manager.StatusJSON(cmd.Context(), worktree)
					if err != nil {
						return err
					}
					_, err = fmt.Fprintln(a.out, string(payload))
					return err
				}
				rows, err := rc.manager.Status(cmd.Context(), worktree)
				if err != nil {
					return err
				}
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "  \tWORKTREE\tPROCESSES\tSTATUS\tBRANCH\tDIR")
				for _, row := range rows {
					var marker string
					if row.Active {
						marker = "★"
					} else {
						marker = " "
					}
					if len(row.Processes) > 0 {
						for pi, p := range row.Processes {
							var status string
							if p.Running && p.Exited {
								status = "● exited"
							} else if p.Running {
								status = "● running"
							} else {
								status = "○ stopped"
							}
							wtName := row.Worktree
							branch := row.Branch
							dir := row.Dir
							m := marker
							if pi > 0 {
								wtName = ""
								branch = ""
								dir = ""
								m = " "
							}
							_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
								m, displayField(wtName), displayField(p.Name), status, displayField(branch), displayField(dir))
						}
					} else {
						var status string
						if row.Running {
							status = "● running"
						} else {
							status = "○ stopped"
						}
						_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
							marker, displayField(row.Worktree), displayField(row.Process), status, displayField(row.Branch), displayField(row.Dir))
					}
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	return cmd
}

func (a *app) newLogsCmd() *cobra.Command {
	var (
		lines   int
		process string
	)
	cmd := &cobra.Command{
		Use:   "logs <worktree>",
		Short: "Show recent tmux pane output for a worktree",
		Long:  "Capture recent output lines from the tmux window for a running worktree process.",
		Example: strings.TrimSpace(`
  wts logs repo-main
  wts logs repo-main --process api
  wts logs ../repo-agent --lines 400
`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if lines <= 0 {
				return fmt.Errorf("--lines must be greater than zero")
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				output, err := rc.manager.Logs(cmd.Context(), args[0], strings.TrimSpace(process), lines)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(a.out, output)
				return err
			})
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 200, "number of log lines")
	cmd.Flags().StringVar(&process, "process", "", "process name (default: first pane)")
	return cmd
}

func (a *app) newPickCmd() *cobra.Command {
	var (
		attach  bool
		process string
		group   string
	)
	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Pick a git worktree and switch",
		Long:  "Open an interactive selector (fzf if installed, fallback prompt otherwise) and switch to the chosen worktree.",
		Example: strings.TrimSpace(`
  wts pick
  wts pick --process demo-script
  wts pick --group dev
  wts pick --attach
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := runOptionsFromFlags(strings.TrimSpace(process), strings.TrimSpace(group), attach)
			if err != nil {
				return err
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				items := rc.manager.ListWorktrees()
				if len(items) == 0 {
					return fmt.Errorf("no git worktrees found (create one with: git worktree add ../branch-name)")
				}
				labels := make([]string, 0, len(items))
				labelToDir := make(map[string]string, len(items))
				for _, wt := range items {
					label := worktreeLabel(wt)
					labels = append(labels, label)
					labelToDir[label] = wt.Dir
				}
				sort.Strings(labels)

				picker := NewPicker(a.in, a.out, a.err)
				selected, err := picker.SelectContext(cmd.Context(), labels)
				if err != nil {
					return err
				}
				return rc.manager.Switch(cmd.Context(), labelToDir[selected], opts)
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	cmd.Flags().StringVar(&group, "group", "", "process group name from config")
	return cmd
}

func (a *app) newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open interactive Bubble Tea TUI",
		Long: strings.TrimSpace(`
Open the interactive TUI for worktree/process handoff.

The TUI lets you move selection across discovered worktrees, choose process
profiles or groups, and start/restart/stop quickly. Multiple processes can run
simultaneously in the same worktree as separate tmux panes, including every
member of a configured group. Groups are defined in .wts.yaml and appear in the
target selector as [group] <name>. Press g to create a group and save it back
to the current repo's .wts.yaml.

Search lists matching targets: use up/down to choose, enter to select, or esc to
cancel. The group editor uses tab to change focus, space to toggle members, enter
to save, and esc to cancel. Ctrl+c quits from any screen. Narrow terminals show a
compact worktree list; errors stay visible on a separate header line.

Shortcuts:
  j/↓      next worktree        h/←    prev target
  k/↑      prev worktree        l/→    next target
  s/enter  start/switch target   a      attach tmux
  r        restart target
  x        stop selected target  /      search target by name
  g        create group in .wts.yaml
  X        stop all in worktree  ?      toggle full help
  q        quit

Exiting TUI does not stop running worktree processes.`),
		Example: strings.TrimSpace(`
  wts tui
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runTUICommand(cmd.Context())
		},
	}
}

func runOptionsFromFlags(process, group string, attach bool) (runtime.RunOptions, error) {
	if process != "" && group != "" {
		return runtime.RunOptions{}, fmt.Errorf("--process and --group cannot be combined")
	}
	return runtime.RunOptions{
		Attach:  attach,
		Process: process,
		Group:   group,
	}, nil
}

func (a *app) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show wts version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.commit != "" {
				_, err := fmt.Fprintf(a.out, "wts %s (%s)\n", a.version, a.commit)
				return err
			}
			_, err := fmt.Fprintf(a.out, "wts %s\n", a.version)
			return err
		},
	}
}

func resolveRepoRoot(ctx context.Context, startDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	if startDir != "" {
		cmd.Dir = startDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", fmt.Errorf("resolve git repo root: %w", err)
		}
		return "", fmt.Errorf("resolve git repo root: %s: %w", msg, err)
	}
	// Git terminates the path with a newline; whitespace can be part of the path.
	root := strings.TrimSuffix(string(out), "\n")
	if root == "" {
		return "", fmt.Errorf("resolve git repo root: empty path")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve git repo root: %w", err)
	}
	return abs, nil
}

func worktreeLabel(wt gitwt.Worktree) string {
	if wt.Branch == "" {
		return fmt.Sprintf("%s  (%s)", displayField(wt.Name), displayField(wt.Dir))
	}
	return fmt.Sprintf("%s [%s]  (%s)", displayField(wt.Name), displayField(wt.Branch), displayField(wt.Dir))
}

// displayField keeps terminal controls and table delimiters in names visible.
func displayField(value string) string {
	if strings.ContainsAny(value, "\\\"") || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsControl(r) }) {
		return strconv.Quote(value)
	}
	return value
}
