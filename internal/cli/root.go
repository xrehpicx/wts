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
	"github.com/xrehpicx/wts/internal/detect"
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
}

type runtimeContext struct {
	project   *model.Project
	repoRoot  string
	worktrees []gitwt.Worktree
	manager   *runtime.Manager
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

	root := &cobra.Command{
		Use:     "wts",
		Aliases: []string{"workswitch", "wks"},
		Short:   "workswitch (wts: worktree switch) process handoff for git worktrees",
		Long: `Process config lives in .wts.yaml.
Worktrees are discovered live from: git worktree list --porcelain.

Switching preempts the previously active worktree process and starts the selected
process in the target worktree.`,
		Example: strings.TrimSpace(`
  wts validate
  wts list
  wts switch repo-main --process api
  wts next --process demo-script
  wts status --json
  wts tui
`),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "path to .wts.yaml")

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

func (a *app) withProject(fn func(*model.Project) error) error {
	project, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	return fn(project)
}

func (a *app) withRuntime(ctx context.Context, fn func(*runtimeContext) error) error {
	return a.withProject(func(project *model.Project) error {
		repoRoot, err := resolveRepoRoot()
		if err != nil {
			return err
		}
		worktrees, err := gitwt.Discover(repoRoot)
		if err != nil {
			return err
		}
		rc := &runtimeContext{
			project:   project,
			repoRoot:  repoRoot,
			worktrees: worktrees,
			manager:   runtime.NewManager(project, repoRoot, worktrees, a.newBackend()),
		}
		return fn(rc)
	})
}

func (a *app) newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .wts.yaml",
		Long:  "Validate the process configuration file and print loaded process count.",
		Example: strings.TrimSpace(`
  wts validate
  wts validate --config ../other/.wts.yaml
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withProject(func(p *model.Project) error {
				_, _ = fmt.Fprintf(a.out, "config valid: %s (%d processes)\n", p.ConfigPath, len(p.Processes))
				return nil
			})
		},
	}
}

func (a *app) newProcessesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "processes",
		Short: "List process profiles from config",
		Long:  "Show process profile names and commands from .wts.yaml.",
		Example: strings.TrimSpace(`
  wts processes
  wts processes --config ../other/.wts.yaml
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withProject(func(p *model.Project) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "PROCESS\tCOMMAND")
				for _, proc := range p.Processes {
					_, _ = fmt.Fprintf(tw, "%s\t%s\n", proc.Name, proc.Command)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKTREE\tBRANCH\tDIR")
				for _, wt := range rc.manager.ListWorktrees() {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", wt.Name, wt.Branch, wt.Dir)
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newSwitchCmd() *cobra.Command {
	return a.newRunCmd("switch", "Switch to a worktree and hand off process", (*runtime.Manager).Switch)
}

func (a *app) newStartCmd() *cobra.Command {
	return a.newRunCmd("start", "Start process in a worktree (additive)", (*runtime.Manager).Start)
}

func (a *app) newRestartCmd() *cobra.Command {
	return a.newRunCmd("restart", "Restart process in a worktree", (*runtime.Manager).Restart)
}

func (a *app) newRunCmd(name, short string, fn func(*runtime.Manager, context.Context, string, runtime.RunOptions) error) *cobra.Command {
	var (
		attach  bool
		process string
	)
	cmd := &cobra.Command{
		Use:   name + " <worktree>",
		Short: short,
		Long: strings.TrimSpace(`
Start or move a process profile to a target worktree.

'switch' preempts: stops the previously active worktree, then starts the process.
'start' is additive: starts the process alongside any already running processes.
'restart' stops and re-starts a specific process.`),
		Example: strings.TrimSpace(fmt.Sprintf(`
  wts %s repo-main
  wts %s ../repo-agent --process demo-script
  wts %s /abs/path/to/worktree --attach
`, name, name, name)),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				if err := fn(rc.manager, cmd.Context(), args[0], runtime.RunOptions{
					Attach:  attach,
					Process: strings.TrimSpace(process),
				}); err != nil {
					return err
				}
				verb := name
				if verb == "start" {
					verb = "switched to"
				} else if verb == "switch" {
					verb = "switched to"
				} else {
					verb += "ed"
				}
				_, _ = fmt.Fprintf(a.out, "✓ %s %s\n", verb, args[0])
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after command")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	return cmd
}

func (a *app) newNextCmd() *cobra.Command {
	var (
		attach  bool
		process string
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Switch to next git worktree in list",
		Long:  "Move to the next discovered worktree and hand off the selected process profile.",
		Example: strings.TrimSpace(`
  wts next
  wts next --process demo-script
  wts next --attach
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.cycleAndSwitch(cmd.Context(), 1, runtime.RunOptions{
				Attach:  attach,
				Process: strings.TrimSpace(process),
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	return cmd
}

func (a *app) newPrevCmd() *cobra.Command {
	var (
		attach  bool
		process string
	)
	cmd := &cobra.Command{
		Use:   "prev",
		Short: "Switch to previous git worktree in list",
		Long:  "Move to the previous discovered worktree and hand off the selected process profile.",
		Example: strings.TrimSpace(`
  wts prev
  wts prev --process api
  wts prev --attach
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.cycleAndSwitch(cmd.Context(), -1, runtime.RunOptions{
				Attach:  attach,
				Process: strings.TrimSpace(process),
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	return cmd
}

func (a *app) cycleAndSwitch(ctx context.Context, delta int, opts runtime.RunOptions) error {
	return a.withRuntime(ctx, func(rc *runtimeContext) error {
		items := rc.manager.ListWorktrees()
		if len(items) == 0 {
			return fmt.Errorf("no git worktrees found (create one with: git worktree add ../branch-name)")
		}

		currentIdx := 0
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
		for i := range items {
			if filepath.Clean(items[i].Dir) == activeDir {
				currentIdx = i
				break
			}
		}

		next := (currentIdx + delta + len(items)) % len(items)
		if err := rc.manager.Switch(ctx, items[next].Dir, opts); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.out, "✓ switched to %s\n", items[next].Name)
		return nil
	})
}

func (a *app) newStopCmd() *cobra.Command {
	var (
		all     bool
		process string
	)
	cmd := &cobra.Command{
		Use:   "stop [worktree]",
		Short: "Stop worktree process(es)",
		Long: strings.TrimSpace(`
Stop process windows managed by wts.

With no arguments it stops the active worktree process.
With a selector it stops only that worktree.
With --process it stops a specific process in the worktree.
With --all it stops all discovered worktree windows.`),
		Example: strings.TrimSpace(`
  wts stop
  wts stop repo-main
  wts stop repo-main --process api
  wts stop /abs/path/to/worktree
  wts stop --all
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("--all cannot be combined with a worktree selector")
			}
			proc := strings.TrimSpace(process)
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				switch {
				case all:
					if err := rc.manager.StopAll(cmd.Context()); err != nil {
						return err
					}
					_, _ = fmt.Fprintln(a.out, "✓ stopped all worktrees")
				case len(args) == 1 && proc != "":
					if err := rc.manager.StopProcess(cmd.Context(), args[0], proc); err != nil {
						return err
					}
					_, _ = fmt.Fprintf(a.out, "✓ stopped %s in %s\n", proc, args[0])
				case len(args) == 1:
					if err := rc.manager.StopWorktree(cmd.Context(), args[0]); err != nil {
						return err
					}
					_, _ = fmt.Fprintf(a.out, "✓ stopped %s\n", args[0])
				default:
					if err := rc.manager.StopActive(cmd.Context()); err != nil {
						return err
					}
					_, _ = fmt.Fprintln(a.out, "✓ stopped active worktree")
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "stop all discovered worktrees")
	cmd.Flags().StringVar(&process, "process", "", "stop a specific process (requires worktree argument)")
	return cmd
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
					_, _ = fmt.Fprintln(a.out, string(payload))
					return nil
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
								m, wtName, p.Name, status, branch, dir)
						}
					} else {
						var status string
						if row.Running {
							status = "● running"
						} else {
							status = "○ stopped"
						}
						_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
							marker, row.Worktree, row.Process, status, row.Branch, row.Dir)
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
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				output, err := rc.manager.Logs(cmd.Context(), args[0], strings.TrimSpace(process), lines)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(a.out, output)
				return nil
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
	)
	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Pick a git worktree and switch",
		Long:  "Open an interactive selector (fzf if installed, fallback prompt otherwise) and switch to the chosen worktree.",
		Example: strings.TrimSpace(`
  wts pick
  wts pick --process demo-script
  wts pick --attach
`),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				selected, err := picker.Select(labels)
				if err != nil {
					return err
				}
				return rc.manager.Switch(cmd.Context(), labelToDir[selected], runtime.RunOptions{
					Attach:  attach,
					Process: strings.TrimSpace(process),
				})
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	cmd.Flags().StringVar(&process, "process", "", "process profile name (default: first process in config)")
	return cmd
}

func (a *app) newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open interactive Bubble Tea TUI",
		Long: strings.TrimSpace(`
Open the interactive TUI for worktree/process handoff.

The TUI lets you move selection across discovered worktrees, choose process
profiles, and start/restart/stop quickly. Multiple processes can run
simultaneously in the same worktree as separate tmux panes.

Shortcuts:
  j/↓      next worktree        h/←    prev process
  k/↑      prev worktree        l/→    next process
  s/enter  start/switch          r      restart process
  x        stop selected process /      search process by name
  X        stop all in worktree  ?      toggle full help
  q        quit

Exiting TUI does not stop running worktree processes.`),
		Example: strings.TrimSpace(`
  wts tui
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withRuntime(cmd.Context(), func(rc *runtimeContext) error {
				m := newTUIModel(rc)
				p := tea.NewProgram(m, tea.WithAltScreen())
				finalModel, err := p.Run()
				if err != nil {
					return err
				}
				if tm, ok := finalModel.(*tuiModel); ok && tm.quitInfo != "" {
					_, _ = fmt.Fprint(a.out, tm.quitInfo)
				}
				return nil
			})
		},
	}
}

func (a *app) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show wts version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.commit != "" {
				_, _ = fmt.Fprintf(a.out, "wts %s (%s)\n", a.version, a.commit)
			} else {
				_, _ = fmt.Fprintf(a.out, "wts %s\n", a.version)
			}
			return nil
		},
	}
}

func resolveRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", fmt.Errorf("resolve git repo root: %w", err)
		}
		return "", fmt.Errorf("resolve git repo root: %s", msg)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("resolve git repo root: empty path")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve git repo root: %w", err)
	}
	return abs, nil
}

func (a *app) newInitCmd() *cobra.Command {
	var (
		force  bool
		dir    string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate .wts.yaml by detecting project type",
		Long: strings.TrimSpace(`
Inspect the current (or specified) directory, detect the project type, and
generate a .wts.yaml with inferred processes.

Built-in detectors:
  nodejs     package.json scripts (auto-detects npm/pnpm/yarn/bun)
  go         cmd/ sub-directories or go run .
  python     manage.py (Django) or pyproject.toml / requirements.txt
  makefile   Makefile targets

Custom detectors can be added as YAML files in:
  ~/.config/wts/detectors/

See 'wts init --help' or docs/detectors.md for the file format.`),
		Example: strings.TrimSpace(`
  wts init
  wts init --dir ../my-project
  wts init --force
  wts init --dry-run
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetDir := dir
			if targetDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("resolve working directory: %w", err)
				}
				targetDir = cwd
			}
			abs, err := filepath.Abs(targetDir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}
			targetDir = abs

			outPath := filepath.Join(targetDir, config.DefaultConfigFile)
			if !force && !dryRun {
				if _, err := os.Stat(outPath); err == nil {
					return fmt.Errorf("%s already exists (use --force to overwrite)", config.DefaultConfigFile)
				}
			}

			configDir := detect.ConfigDir()
			result, err := detect.Run(targetDir, configDir)
			if err != nil {
				return fmt.Errorf("detection failed: %w", err)
			}

			var procs []model.Process
			detectedType := "unknown"
			if result != nil {
				detectedType = result.Type
				for _, p := range result.Processes {
					procs = append(procs, model.Process{
						Name:    p.Name,
						Command: p.Command,
					})
				}
			}
			if len(procs) == 0 {
				procs = append(procs, model.Process{
					Name:    "dev",
					Command: "echo 'replace with your dev command'",
				})
			}

			cfg := model.Config{
				Version: model.CurrentVersion,
				Defaults: model.Defaults{
					StopTimeoutSec: model.DefaultStopTimeout,
					Shell:          model.DefaultShell,
				},
				Processes: procs,
			}

			yamlData, err := marshalConfig(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			if result != nil {
				_, _ = fmt.Fprintf(a.out, "✓ Detected %s project (%d processes)\n", detectedType, len(procs))
			} else {
				_, _ = fmt.Fprintln(a.out, "  No project type detected — generating minimal config")
			}

			if dryRun {
				_, _ = fmt.Fprintf(a.out, "\n%s", string(yamlData))
				return nil
			}

			if err := os.WriteFile(outPath, yamlData, 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			_, _ = fmt.Fprintf(a.out, "  Written %s\n", outPath)

			tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "\n  PROCESS\tCOMMAND")
			for _, p := range procs {
				_, _ = fmt.Fprintf(tw, "  %s\t%s\n", p.Name, p.Command)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing "+config.DefaultConfigFile)
	cmd.Flags().StringVar(&dir, "dir", "", "project directory (default: current working directory)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print generated config without writing")
	return cmd
}

func marshalConfig(cfg model.Config) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("version: %d\n", cfg.Version))
	b.WriteString("defaults:\n")
	b.WriteString(fmt.Sprintf("  stop_timeout_sec: %d\n", cfg.Defaults.StopTimeoutSec))
	b.WriteString(fmt.Sprintf("  shell: %s\n", cfg.Defaults.Shell))
	b.WriteString("processes:\n")
	for _, p := range cfg.Processes {
		b.WriteString(fmt.Sprintf("  - name: %s\n", p.Name))
		if strings.ContainsAny(p.Command, "\"'${}|&;<>()") {
			b.WriteString(fmt.Sprintf("    command: %q\n", p.Command))
		} else {
			b.WriteString(fmt.Sprintf("    command: %s\n", p.Command))
		}
	}
	return []byte(b.String()), nil
}

func worktreeLabel(wt gitwt.Worktree) string {
	if wt.Branch == "" {
		return fmt.Sprintf("%s  (%s)", wt.Name, wt.Dir)
	}
	return fmt.Sprintf("%s [%s]  (%s)", wt.Name, wt.Branch, wt.Dir)
}
