package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/xrehpicx/wts/internal/config"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/state"
	"github.com/xrehpicx/wts/internal/tmux"
)

type app struct {
	version    string
	configPath string
	statePath  string
	in         io.Reader
	out        io.Writer
	err        io.Writer

	newBackend func() runtime.Backend
}

type runtimeContext struct {
	project *model.Project
	store   *state.Store
	file    *state.File
	repo    *state.RepoState
	manager *runtime.Manager
}

func NewRootCmd(version string) *cobra.Command {
	a := &app{
		version: version,
		in:      os.Stdin,
		out:     os.Stdout,
		err:     os.Stderr,
		newBackend: func() runtime.Backend {
			return tmux.NewClient("tmux")
		},
	}

	root := &cobra.Command{
		Use:     "wts",
		Aliases: []string{"workswitch", "wks"},
		Short:   "workswitch (wts: worktree switch) for moving processes across worktrees",
		Long: `Process config lives in .wts.yaml.
Worktree directory assignments live in ~/.workswitch/state.yaml.

Use this when AI agents/devs work in multiple git worktrees and you want
fast process handoff with per-group preemption.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "path to .wts.yaml")
	root.PersistentFlags().StringVar(&a.statePath, "state", "", "path to state file (default ~/.workswitch/state.yaml)")

	root.AddCommand(a.newValidateCmd())
	root.AddCommand(a.newProcessesCmd())
	root.AddCommand(a.newListCmd())
	root.AddCommand(a.newAddCmd())
	root.AddCommand(a.newRemoveCmd())
	root.AddCommand(a.newAssignCmd())
	root.AddCommand(a.newGroupCmd())
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

func Execute(version string) error {
	return NewRootCmd(version).Execute()
}

func (a *app) withRuntime(ctx context.Context, fn func(*runtimeContext) error) error {
	project, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return err
	}
	store, err := state.NewStore(a.statePath)
	if err != nil {
		return err
	}
	f, err := store.Load()
	if err != nil {
		return err
	}
	repo, _, err := f.EnsureRepo(repoRoot)
	if err != nil {
		return err
	}
	if err := repo.Normalize(); err != nil {
		return err
	}

	rc := &runtimeContext{
		project: project,
		store:   store,
		file:    f,
		repo:    repo,
		manager: runtime.NewManager(project, repo, a.newBackend()),
	}
	return fn(rc)
}

func (a *app) newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .wts.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(a.configPath)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(a.out, "config valid: %s (%d processes)\n", p.ConfigPath, len(p.Processes))
			return nil
		},
	}
}

func (a *app) newProcessesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "processes",
		Short: "List process profiles from config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "PROCESS\tGROUP\tCOMMAND")
				for _, name := range rc.project.ProcessNames() {
					p, _ := rc.project.Process(name)
					group := model.EffectiveGroup(p, "", name)
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, group, p.Command)
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured worktrees for this repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKTREE\tPROCESS\tGROUP\tDIR")
				for _, wt := range rc.manager.ListWorktrees() {
					proc, err := rc.project.Process(wt.Process)
					if err != nil {
						return err
					}
					group := model.EffectiveGroup(proc, wt.Group, wt.Name)
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", wt.Name, wt.Process, group, wt.Dir)
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newAddCmd() *cobra.Command {
	var process, name, group string
	cmd := &cobra.Command{
		Use:   "add <dir>",
		Short: "Add a worktree directory assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if process == "" {
					return fmt.Errorf("--process is required")
				}
				if _, err := rc.project.Process(process); err != nil {
					return err
				}
				dir, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				info, err := os.Stat(dir)
				if err != nil || !info.IsDir() {
					return fmt.Errorf("dir %q is not a directory", dir)
				}
				if name == "" {
					name = autoWorktreeName(dir, rc.repo)
				}
				if existing, _ := rc.repo.Worktree(name); existing != nil {
					return fmt.Errorf("worktree %q already exists", name)
				}
				rc.repo.Worktrees = append(rc.repo.Worktrees, state.Worktree{Name: name, Dir: dir, Process: process, Group: strings.TrimSpace(group)})
				if err := rc.repo.Normalize(); err != nil {
					return err
				}
				if err := rc.store.Save(rc.file); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(a.out, "added worktree %s -> %s (%s)\n", name, dir, process)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&process, "process", "", "process profile name")
	cmd.Flags().StringVar(&name, "name", "", "worktree alias name")
	cmd.Flags().StringVar(&group, "group", "", "optional group override")
	return cmd
}

func (a *app) newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <worktree>",
		Short: "Remove a worktree assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if !rc.repo.RemoveWorktree(args[0]) {
					return fmt.Errorf("worktree %q not found", args[0])
				}
				if err := rc.store.Save(rc.file); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(a.out, "removed worktree %s\n", args[0])
				return nil
			})
		},
	}
}

func (a *app) newAssignCmd() *cobra.Command {
	var process string
	cmd := &cobra.Command{
		Use:   "assign <worktree>",
		Short: "Assign a process profile to a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if process == "" {
					return fmt.Errorf("--process is required")
				}
				if _, err := rc.project.Process(process); err != nil {
					return err
				}
				wt, _ := rc.repo.Worktree(args[0])
				if wt == nil {
					return fmt.Errorf("worktree %q not found", args[0])
				}
				wt.Process = process
				if err := rc.store.Save(rc.file); err != nil {
					return err
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&process, "process", "", "process profile name")
	return cmd
}

func (a *app) newGroupCmd() *cobra.Command {
	var set string
	var clear bool
	cmd := &cobra.Command{
		Use:   "group <worktree>",
		Short: "Set or clear group override for a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear && set != "" {
				return fmt.Errorf("--set and --clear are mutually exclusive")
			}
			if !clear && set == "" {
				return fmt.Errorf("provide --set <group> or --clear")
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				wt, _ := rc.repo.Worktree(args[0])
				if wt == nil {
					return fmt.Errorf("worktree %q not found", args[0])
				}
				if clear {
					wt.Group = ""
				} else {
					wt.Group = strings.TrimSpace(set)
				}
				return rc.store.Save(rc.file)
			})
		},
	}
	cmd.Flags().StringVar(&set, "set", "", "set group override")
	cmd.Flags().BoolVar(&clear, "clear", false, "clear group override")
	return cmd
}

func (a *app) newSwitchCmd() *cobra.Command {
	return a.newRunCmd("switch", "Switch to a worktree", (*runtime.Manager).Switch)
}
func (a *app) newStartCmd() *cobra.Command {
	return a.newRunCmd("start", "Start process for a worktree", (*runtime.Manager).Start)
}
func (a *app) newRestartCmd() *cobra.Command {
	return a.newRunCmd("restart", "Restart process for a worktree", (*runtime.Manager).Restart)
}

func (a *app) newRunCmd(name, short string, fn func(*runtime.Manager, context.Context, string, runtime.SwitchOptions) error) *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   name + " <worktree>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if err := fn(rc.manager, cmd.Context(), args[0], runtime.SwitchOptions{Attach: attach}); err != nil {
					return err
				}
				for i := range rc.repo.Worktrees {
					if rc.repo.Worktrees[i].Name == args[0] {
						rc.repo.Selected = i
						break
					}
				}
				return rc.store.Save(rc.file)
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after command")
	return cmd
}

func (a *app) newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Switch to next worktree in list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.cycleAndSwitch(cmd.Context(), 1)
		},
	}
}

func (a *app) newPrevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prev",
		Short: "Switch to previous worktree in list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.cycleAndSwitch(cmd.Context(), -1)
		},
	}
}

func (a *app) cycleAndSwitch(ctx context.Context, delta int) error {
	return a.withRuntime(ctx, func(rc *runtimeContext) error {
		if len(rc.repo.Worktrees) == 0 {
			return fmt.Errorf("no worktrees configured; use wts add")
		}
		rc.repo.Selected = (rc.repo.Selected + delta + len(rc.repo.Worktrees)) % len(rc.repo.Worktrees)
		name := rc.repo.Worktrees[rc.repo.Selected].Name
		if err := rc.manager.Switch(ctx, name, runtime.SwitchOptions{}); err != nil {
			return err
		}
		return rc.store.Save(rc.file)
	})
}

func (a *app) newStopCmd() *cobra.Command {
	var group string
	var all bool
	cmd := &cobra.Command{
		Use:   "stop [worktree]",
		Short: "Stop worktree process(es)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && (group != "" || len(args) > 0) {
				return fmt.Errorf("--all cannot be combined with other stop selectors")
			}
			if group != "" && len(args) > 0 {
				return fmt.Errorf("provide either worktree arg or --group")
			}
			if !all && group == "" && len(args) == 0 {
				return fmt.Errorf("provide worktree, --group, or --all")
			}
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				switch {
				case all:
					return rc.manager.StopAll(cmd.Context())
				case group != "":
					return rc.manager.StopGroup(cmd.Context(), group)
				default:
					return rc.manager.StopWorktree(cmd.Context(), args[0])
				}
			})
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "stop active worktree in group")
	cmd.Flags().BoolVar(&all, "all", false, "stop all worktrees")
	return cmd
}

func (a *app) newStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status [worktree]",
		Short: "Show worktree runtime status",
		Args:  cobra.MaximumNArgs(1),
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
					_, _ = fmt.Fprintln(a.out, string(payload))
					return nil
				}
				rows, err := rc.manager.Status(cmd.Context(), worktree)
				if err != nil {
					return err
				}
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKTREE\tPROCESS\tGROUP\tRUNNING\tACTIVE\tDIR")
				for _, row := range rows {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%t\t%t\t%s\n", row.Worktree, row.Process, row.Group, row.Running, row.Active, row.Dir)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	return cmd
}

func (a *app) newLogsCmd() *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "logs <worktree>",
		Short: "Show recent tmux pane output for a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				output, err := rc.manager.Logs(cmd.Context(), args[0], lines)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(a.out, output)
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 200, "number of log lines")
	return cmd
}

func (a *app) newPickCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Pick a worktree and switch",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				names := make([]string, 0, len(rc.repo.Worktrees))
				for _, wt := range rc.repo.Worktrees {
					names = append(names, wt.Name)
				}
				sort.Strings(names)
				if len(names) == 0 {
					return fmt.Errorf("no worktrees configured; use wts add")
				}
				picker := NewPicker(a.in, a.out, a.err)
				selected, err := picker.Select(names)
				if err != nil {
					return err
				}
				if err := rc.manager.Switch(cmd.Context(), selected, runtime.SwitchOptions{Attach: attach}); err != nil {
					return err
				}
				for i := range rc.repo.Worktrees {
					if rc.repo.Worktrees[i].Name == selected {
						rc.repo.Selected = i
						break
					}
				}
				return rc.store.Save(rc.file)
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	return cmd
}

func (a *app) newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open interactive Bubble Tea TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				m := newTUIModel(rc)
				p := tea.NewProgram(m, tea.WithAltScreen())
				_, err := p.Run()
				return err
			})
		},
	}
}

func (a *app) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show wts version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(a.out, "wts %s\n", a.version)
			return nil
		},
	}
}

func resolveRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return filepath.Abs(root)
		}
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", fmt.Errorf("resolve cwd: %w", cwdErr)
	}
	return filepath.Abs(cwd)
}

func autoWorktreeName(dir string, repo *state.RepoState) string {
	base := strings.TrimSpace(filepath.Base(dir))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "worktree"
	}
	name := base
	idx := 1
	for {
		if existing, _ := repo.Worktree(name); existing == nil {
			return name
		}
		idx++
		name = fmt.Sprintf("%s-%d", base, idx)
	}
}
