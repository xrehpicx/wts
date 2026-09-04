package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/xrehpicx/wts/internal/config"
	"github.com/xrehpicx/wts/internal/detect"
	"github.com/xrehpicx/wts/internal/model"
)

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
		Args: cobra.NoArgs,
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
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("inspect existing config: %w", err)
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

			yamlData, err := config.Marshal(cfg)
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

			if _, err := config.Save(outPath, cfg); err != nil {
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
