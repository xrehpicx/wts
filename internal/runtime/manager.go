package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/tmux"
)

type Backend interface {
	EnsureTmux(ctx context.Context) error
	EnsureSession(ctx context.Context, session string) error
	HasWindow(ctx context.Context, session, window string) (bool, error)
	StartWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string) error
	StopWindow(ctx context.Context, session, window string, timeout time.Duration) error
	SetSessionOption(ctx context.Context, session, key, value string) error
	GetSessionOption(ctx context.Context, session, key string) (string, error)
	CapturePane(ctx context.Context, session, window string, lines int) (string, error)
	PaneCurrentCommand(ctx context.Context, session, window string) (string, error)
	Attach(ctx context.Context, session, window string) error
}

type Manager struct {
	project   *model.Project
	worktrees []gitwt.Worktree
	backend   Backend
	session   string
}

type RunOptions struct {
	Attach  bool
	Process string
}

type StatusRow struct {
	Worktree string `json:"worktree"`
	Process  string `json:"process"`
	Running  bool   `json:"running"`
	Exited   bool   `json:"exited"`
	Active   bool   `json:"active"`
	Branch   string `json:"branch"`
	Dir      string `json:"dir"`
}

func NewManager(project *model.Project, repoRoot string, worktrees []gitwt.Worktree, backend Backend) *Manager {
	items := make([]gitwt.Worktree, len(worktrees))
	copy(items, worktrees)
	return &Manager{
		project:   project,
		worktrees: items,
		backend:   backend,
		session:   tmux.SessionName(repoRoot),
	}
}

func (m *Manager) Session() string {
	return m.session
}

func (m *Manager) ActiveProcess(ctx context.Context) string {
	name, _ := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey())
	return name
}

func (m *Manager) ListWorktrees() []gitwt.Worktree {
	items := make([]gitwt.Worktree, len(m.worktrees))
	copy(items, m.worktrees)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Dir < items[j].Dir
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (m *Manager) Switch(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, false)
}

func (m *Manager) Start(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, false)
}

func (m *Manager) Restart(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, true)
}

func (m *Manager) activate(ctx context.Context, worktree string, opts RunOptions, forceRestart bool) error {
	wt, err := m.resolveWorktree(worktree)
	if err != nil {
		return err
	}
	proc, err := m.resolveProcess(opts.Process)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}

	activeDir, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return err
	}
	if activeDir != "" && filepath.Clean(activeDir) != filepath.Clean(wt.Dir) {
		if err := m.stopWorktreeProcessByDir(ctx, activeDir); err != nil {
			return fmt.Errorf("stop previous worktree %q: %w", activeDir, err)
		}
	}

	if forceRestart {
		if err := m.stopWorktreeProcess(ctx, wt); err != nil {
			return fmt.Errorf("restart worktree %q: %w", wt.Name, err)
		}
	}

	running, err := m.isRunning(ctx, wt)
	if err != nil {
		return err
	}
	if running {
		// If a different process is selected, restart with the new one.
		currentProc, _ := m.backend.GetSessionOption(ctx, m.session, tmux.ProcessOptionKey(wt.Dir))
		if currentProc != proc.Name {
			if err := m.stopWorktreeProcess(ctx, wt); err != nil {
				return fmt.Errorf("stop old process in %q: %w", wt.Name, err)
			}
			running = false
		}
	}
	if !running {
		if err := m.startWorktreeProcess(ctx, wt, proc); err != nil {
			return err
		}
	}

	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), wt.Dir); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), proc.Name); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ProcessOptionKey(wt.Dir), proc.Name); err != nil {
		return err
	}

	if opts.Attach {
		if err := m.backend.Attach(ctx, m.session, tmux.WindowName(wt.Dir)); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) StopWorktree(ctx context.Context, worktree string) error {
	wt, err := m.resolveWorktree(worktree)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	if err := m.stopWorktreeProcess(ctx, wt); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ProcessOptionKey(wt.Dir), ""); err != nil {
		return err
	}

	active, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return err
	}
	if filepath.Clean(active) == filepath.Clean(wt.Dir) {
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), ""); err != nil {
			return err
		}
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), ""); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) StopActive(ctx context.Context) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	active, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return err
	}
	if active == "" {
		return nil
	}
	if err := m.stopWorktreeProcessByDir(ctx, active); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ProcessOptionKey(active), ""); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), ""); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	for i := range m.worktrees {
		wt := &m.worktrees[i]
		if err := m.stopWorktreeProcess(ctx, wt); err != nil {
			return err
		}
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.ProcessOptionKey(wt.Dir), ""); err != nil {
			return err
		}
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), ""); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Logs(ctx context.Context, worktree string, lines int) (string, error) {
	wt, err := m.resolveWorktree(worktree)
	if err != nil {
		return "", err
	}
	if err := m.ensureReady(ctx); err != nil {
		return "", err
	}
	running, err := m.isRunning(ctx, wt)
	if err != nil {
		return "", err
	}
	if !running {
		return "", fmt.Errorf("worktree %q is not running", wt.Name)
	}
	return m.backend.CapturePane(ctx, m.session, tmux.WindowName(wt.Dir), lines)
}

func (m *Manager) Status(ctx context.Context, worktree string) ([]StatusRow, error) {
	if err := m.ensureReady(ctx); err != nil {
		return nil, err
	}

	targetDir := ""
	if worktree != "" {
		wt, err := m.resolveWorktree(worktree)
		if err != nil {
			return nil, err
		}
		targetDir = filepath.Clean(wt.Dir)
	}

	activeDir, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return nil, err
	}
	activeDir = filepath.Clean(activeDir)
	activeProc, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey())
	if err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0, len(m.worktrees))
	for i := range m.worktrees {
		wt := &m.worktrees[i]
		if targetDir != "" && filepath.Clean(wt.Dir) != targetDir {
			continue
		}
		running, err := m.isRunning(ctx, wt)
		if err != nil {
			return nil, err
		}
		exited := false
		if running {
			exited = m.isProcessExited(ctx, wt)
		}
		active := filepath.Clean(wt.Dir) == activeDir
		process := m.defaultProcessName()
		if active && activeProc != "" {
			process = activeProc
		} else {
			procByWorktree, err := m.backend.GetSessionOption(ctx, m.session, tmux.ProcessOptionKey(wt.Dir))
			if err != nil {
				return nil, err
			}
			if procByWorktree != "" {
				process = procByWorktree
			}
		}

		rows = append(rows, StatusRow{
			Worktree: wt.Name,
			Process:  process,
			Running:  running,
			Exited:   exited,
			Active:   active,
			Branch:   wt.Branch,
			Dir:      wt.Dir,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Worktree == rows[j].Worktree {
			return rows[i].Dir < rows[j].Dir
		}
		return rows[i].Worktree < rows[j].Worktree
	})
	if worktree != "" && len(rows) == 0 {
		return nil, fmt.Errorf("worktree %q not found", worktree)
	}
	return rows, nil
}

func (m *Manager) StatusJSON(ctx context.Context, worktree string) ([]byte, error) {
	rows, err := m.Status(ctx, worktree)
	if err != nil {
		return nil, err
	}
	if worktree != "" {
		return json.MarshalIndent(rows[0], "", "  ")
	}
	return json.MarshalIndent(rows, "", "  ")
}

func (m *Manager) defaultProcessName() string {
	if len(m.project.Processes) == 0 {
		return ""
	}
	return m.project.Processes[0].Name
}

func (m *Manager) ensureReady(ctx context.Context) error {
	if err := m.backend.EnsureTmux(ctx); err != nil {
		return err
	}
	if err := m.backend.EnsureSession(ctx, m.session); err != nil {
		return err
	}
	return nil
}

func (m *Manager) resolveWorktree(selector string) (*gitwt.Worktree, error) {
	return gitwt.Resolve(m.worktrees, selector)
}

func (m *Manager) resolveProcess(name string) (*model.Process, error) {
	processName := name
	if processName == "" {
		processName = m.defaultProcessName()
	}
	if processName == "" {
		return nil, fmt.Errorf("no process profiles configured")
	}
	return m.project.Process(processName)
}

func (m *Manager) startWorktreeProcess(ctx context.Context, wt *gitwt.Worktree, proc *model.Process) error {
	if err := m.backend.StartWindowCommand(
		ctx,
		m.session,
		tmux.WindowName(wt.Dir),
		wt.Dir,
		m.project.Defaults.Shell,
		proc.Command,
		proc.Env,
	); err != nil {
		return fmt.Errorf("start worktree %q: %w", wt.Name, err)
	}
	return nil
}

func (m *Manager) stopWorktreeProcess(ctx context.Context, wt *gitwt.Worktree) error {
	return m.stopWorktreeProcessByDir(ctx, wt.Dir)
}

func (m *Manager) stopWorktreeProcessByDir(ctx context.Context, worktreeDir string) error {
	timeout := time.Duration(m.project.Defaults.StopTimeoutSec) * time.Second
	if err := m.backend.StopWindow(ctx, m.session, tmux.WindowName(worktreeDir), timeout); err != nil {
		return fmt.Errorf("stop worktree %q: %w", worktreeDir, err)
	}
	return nil
}

func (m *Manager) isRunning(ctx context.Context, wt *gitwt.Worktree) (bool, error) {
	return m.backend.HasWindow(ctx, m.session, tmux.WindowName(wt.Dir))
}

// shellNames are the common shell base names that indicate the process has
// exited and the pane fell back to an interactive prompt.
var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"dash": true, "ksh": true, "csh": true, "tcsh": true,
}

func (m *Manager) isProcessExited(ctx context.Context, wt *gitwt.Worktree) bool {
	cmd, err := m.backend.PaneCurrentCommand(ctx, m.session, tmux.WindowName(wt.Dir))
	if err != nil || cmd == "" {
		return false
	}
	return shellNames[cmd]
}
