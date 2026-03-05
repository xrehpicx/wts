package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/xrehpicx/wks/internal/config"
	"github.com/xrehpicx/wks/internal/model"
	"github.com/xrehpicx/wks/internal/runtime"
	"github.com/xrehpicx/wks/internal/tmux"
)

type app struct {
	version    string
	configPath string
	in         io.Reader
	out        io.Writer
	err        io.Writer

	newBackend func() runtime.Backend
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
		Use:           "wks",
		Aliases:       []string{"workswitch"},
		Short:         "Switch managed workspace processes via tmux",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "path to .workswitch.yaml")

	root.AddCommand(a.newListCmd())
	root.AddCommand(a.newSwitchCmd())
	root.AddCommand(a.newStartCmd())
	root.AddCommand(a.newRestartCmd())
	root.AddCommand(a.newStopCmd())
	root.AddCommand(a.newStatusCmd())
	root.AddCommand(a.newLogsCmd())
	root.AddCommand(a.newPickCmd())
	root.AddCommand(a.newValidateCmd())
	root.AddCommand(a.newVersionCmd())

	return root
}

func Execute(version string) error {
	return NewRootCmd(version).Execute()
}

func (a *app) loadProject() (*model.Project, error) {
	project, err := config.Load(a.configPath)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (a *app) withManager(ctx context.Context, fn func(*runtime.Manager, *model.Project) error) error {
	project, err := a.loadProject()
	if err != nil {
		return err
	}
	manager := runtime.NewManager(project, a.newBackend())
	return fn(manager, project)
}

func (a *app) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				rows := manager.ListWorkspaces()
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKSPACE\tGROUP\tDIR\tCOMMAND")
				for _, ws := range rows {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ws.Name, ws.EffectiveGroup, ws.ResolvedDir, ws.Command)
				}
				return tw.Flush()
			})
		},
	}
}

func (a *app) newSwitchCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "switch <workspace>",
		Short: "Switch active workspace within its group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				return manager.Switch(cmd.Context(), args[0], runtime.SwitchOptions{Attach: attach})
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	return cmd
}

func (a *app) newStartCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "start <workspace>",
		Short: "Start workspace process and make it active for its group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				return manager.Start(cmd.Context(), args[0], runtime.SwitchOptions{Attach: attach})
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after starting")
	return cmd
}

func (a *app) newRestartCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "restart <workspace>",
		Short: "Restart workspace process and make it active for its group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				return manager.Restart(cmd.Context(), args[0], runtime.SwitchOptions{Attach: attach})
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after restarting")
	return cmd
}

func (a *app) newStopCmd() *cobra.Command {
	var group string
	var all bool
	cmd := &cobra.Command{
		Use:   "stop [workspace]",
		Short: "Stop workspace process(es)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 || group != "" {
					return fmt.Errorf("--all cannot be combined with workspace or --group")
				}
			}
			if group != "" && len(args) > 0 {
				return fmt.Errorf("provide either workspace arg or --group")
			}
			if !all && group == "" && len(args) == 0 {
				return fmt.Errorf("provide workspace, --group, or --all")
			}
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				switch {
				case all:
					return manager.StopAll(cmd.Context())
				case group != "":
					return manager.StopGroup(cmd.Context(), group)
				default:
					return manager.StopWorkspace(cmd.Context(), args[0])
				}
			})
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "stop active workspace in group")
	cmd.Flags().BoolVar(&all, "all", false, "stop all running workspace processes")
	return cmd
}

func (a *app) newStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status [workspace]",
		Short: "Show workspace runtime status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace := ""
			if len(args) == 1 {
				workspace = args[0]
			}
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				if jsonOut {
					payload, err := manager.StatusJSON(cmd.Context(), workspace)
					if err != nil {
						return err
					}
					_, _ = fmt.Fprintln(a.out, string(payload))
					return nil
				}
				rows, err := manager.Status(cmd.Context(), workspace)
				if err != nil {
					return err
				}
				tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "WORKSPACE\tGROUP\tRUNNING\tACTIVE\tDIR")
				for _, row := range rows {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%t\t%t\t%s\n", row.Workspace, row.Group, row.Running, row.Active, row.Dir)
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
		Use:   "logs <workspace>",
		Short: "Show recent tmux pane output for a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, _ *model.Project) error {
				output, err := manager.Logs(cmd.Context(), args[0], lines)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(a.out, output)
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 200, "number of log lines to capture")
	return cmd
}

func (a *app) newPickCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Interactively pick and switch to a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withManager(cmd.Context(), func(manager *runtime.Manager, project *model.Project) error {
				names := project.WorkspaceNames()
				sort.Strings(names)
				picker := NewPicker(a.in, a.out, a.err)
				selected, err := picker.Select(names)
				if err != nil {
					return err
				}
				selected = strings.TrimSpace(selected)
				if selected == "" {
					return fmt.Errorf("no workspace selected")
				}
				return manager.Switch(cmd.Context(), selected, runtime.SwitchOptions{Attach: attach})
			})
		},
	}
	cmd.Flags().BoolVar(&attach, "attach", false, "attach/focus tmux after switching")
	return cmd
}

func (a *app) newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .workswitch.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(a.out, "config valid: %s (%d workspaces)\n", project.ConfigPath, len(project.Workspaces))
			return nil
		},
	}
}

func (a *app) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show wks version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(a.out, "wks %s\n", a.version)
			return nil
		},
	}
}
