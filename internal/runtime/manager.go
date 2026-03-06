package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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

	SetPaneTitle(ctx context.Context, session, window, title string) error
	SplitWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string, paneTitle string) error
	ListPanes(ctx context.Context, session, window string) ([]tmux.PaneInfo, error)
	StopPane(ctx context.Context, paneID string, timeout time.Duration) error
	CapturePaneByID(ctx context.Context, paneID string, lines int) (string, error)
	PaneExitedByPID(ctx context.Context, pid string) bool
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

type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Exited  bool   `json:"exited"`
	PaneID  string `json:"-"`
}

type StatusRow struct {
	Worktree  string          `json:"worktree"`
	Process   string          `json:"process"`
	Processes []ProcessStatus `json:"processes,omitempty"`
	Running   bool            `json:"running"`
	Exited    bool            `json:"exited"`
	Active    bool            `json:"active"`
	Branch    string          `json:"branch"`
	Dir       string          `json:"dir"`
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

// Switch preempts: stops all processes in the previous active worktree,
// then starts the selected process in the target worktree (additive).
func (m *Manager) Switch(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, false, true)
}

// Start is additive: starts the selected process in the target worktree
// without stopping processes in other worktrees.
func (m *Manager) Start(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, false, false)
}

func (m *Manager) Restart(ctx context.Context, worktree string, opts RunOptions) error {
	return m.activate(ctx, worktree, opts, true, false)
}

func (m *Manager) activate(ctx context.Context, worktree string, opts RunOptions, forceRestart, preemptive bool) error {
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

	if preemptive {
		activeDir, err := m.backend.GetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey())
		if err != nil {
			return err
		}
		if activeDir != "" && filepath.Clean(activeDir) != filepath.Clean(wt.Dir) {
			if err := m.stopWorktreeProcessByDir(ctx, activeDir); err != nil {
				return fmt.Errorf("stop previous worktree %q: %w", activeDir, err)
			}
		}
	}

	windowName := tmux.WindowName(wt.Dir)
	windowExists, err := m.backend.HasWindow(ctx, m.session, windowName)
	if err != nil {
		return err
	}

	paneTitle := tmux.ProcessPaneTitle(proc.Name)

	if windowExists {
		// Check if this specific process is already running as a pane.
		pane := m.findProcessPane(ctx, wt, proc.Name)

		if forceRestart && pane != nil {
			timeout := time.Duration(m.project.Defaults.StopTimeoutSec) * time.Second
			if err := m.backend.StopPane(ctx, pane.ID, timeout); err != nil {
				return fmt.Errorf("stop process %q in %q: %w", proc.Name, wt.Name, err)
			}
			pane = nil
		}

		if pane == nil {
			if err := m.backend.SplitWindowCommand(
				ctx, m.session, windowName,
				wt.Dir, m.project.Defaults.Shell,
				proc.Command, proc.Env, paneTitle,
			); err != nil {
				return fmt.Errorf("add process %q to %q: %w", proc.Name, wt.Name, err)
			}
		}
	} else {
		if err := m.backend.StartWindowCommand(
			ctx, m.session, windowName,
			wt.Dir, m.project.Defaults.Shell,
			proc.Command, proc.Env,
		); err != nil {
			return fmt.Errorf("start worktree %q: %w", wt.Name, err)
		}
		if err := m.backend.SetPaneTitle(ctx, m.session, windowName, paneTitle); err != nil {
			return fmt.Errorf("set pane title in %q: %w", wt.Name, err)
		}
	}

	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), wt.Dir); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), proc.Name); err != nil {
		return err
	}

	if opts.Attach {
		if err := m.backend.Attach(ctx, m.session, windowName); err != nil {
			return err
		}
	}

	return nil
}

// findProcessPane returns the PaneInfo for a running process in a worktree, or nil.
func (m *Manager) findProcessPane(ctx context.Context, wt *gitwt.Worktree, processName string) *tmux.PaneInfo {
	panes, err := m.backend.ListPanes(ctx, m.session, tmux.WindowName(wt.Dir))
	if err != nil {
		return nil
	}
	title := tmux.ProcessPaneTitle(processName)
	for i := range panes {
		if panes[i].Title == title {
			return &panes[i]
		}
	}
	// Fallback: legacy pane without wts: prefix (started before multi-process).
	// If there's exactly one pane with no wts: title, assume it's the process.
	if len(panes) == 1 && tmux.ProcessFromPaneTitle(panes[0].Title) == "" {
		return &panes[0]
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

// StopProcess stops a specific process in a worktree, leaving other processes running.
func (m *Manager) StopProcess(ctx context.Context, worktree, processName string) error {
	wt, err := m.resolveWorktree(worktree)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	pane := m.findProcessPane(ctx, wt, processName)
	if pane == nil {
		return fmt.Errorf("process %q not running in worktree %q", processName, wt.Name)
	}
	timeout := time.Duration(m.project.Defaults.StopTimeoutSec) * time.Second
	return m.backend.StopPane(ctx, pane.ID, timeout)
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
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveWorktreeOptionKey(), ""); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.ActiveProcessOptionKey(), ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Logs(ctx context.Context, worktree string, processName string, lines int) (string, error) {
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

	if processName != "" {
		pane := m.findProcessPane(ctx, wt, processName)
		if pane == nil {
			return "", fmt.Errorf("process %q not running in worktree %q", processName, wt.Name)
		}
		return m.backend.CapturePaneByID(ctx, pane.ID, lines)
	}

	// Default: capture from the whole window (first pane).
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

		windowName := tmux.WindowName(wt.Dir)
		windowExists, err := m.backend.HasWindow(ctx, m.session, windowName)
		if err != nil {
			return nil, err
		}

		active := filepath.Clean(wt.Dir) == activeDir
		var procs []ProcessStatus
		anyRunning := false
		allExited := true
		primaryProcess := m.defaultProcessName()

		if windowExists {
			panes, err := m.backend.ListPanes(ctx, m.session, windowName)
			if err == nil && len(panes) > 0 {
				for _, pane := range panes {
					procName := tmux.ProcessFromPaneTitle(pane.Title)
					if procName == "" {
						// Legacy pane without wts title — use active process as fallback.
						if active && activeProc != "" {
							procName = activeProc
						} else {
							procName = primaryProcess
						}
					}
					exited := false
					if pane.PID != "" {
						exited = m.backend.PaneExitedByPID(ctx, pane.PID)
					} else {
						exited = tmux.IsShellCommand(pane.Command)
					}
					procs = append(procs, ProcessStatus{
						Name:    procName,
						Running: true,
						Exited:  exited,
						PaneID:  pane.ID,
					})
					anyRunning = true
					if !exited {
						allExited = false
					}
				}
			} else {
				anyRunning = true
			}
		}

		if len(procs) > 0 {
			primaryProcess = procs[0].Name
		} else if active && activeProc != "" {
			primaryProcess = activeProc
		}

		// Build process name summary for backward compat.
		procNames := make([]string, 0, len(procs))
		for _, p := range procs {
			procNames = append(procNames, p.Name)
		}
		processStr := primaryProcess
		if len(procNames) > 1 {
			processStr = strings.Join(procNames, ", ")
		} else if len(procNames) == 1 {
			processStr = procNames[0]
		}

		rows = append(rows, StatusRow{
			Worktree:  wt.Name,
			Process:   processStr,
			Processes: procs,
			Running:   anyRunning,
			Exited:    anyRunning && allExited,
			Active:    active,
			Branch:    wt.Branch,
			Dir:       wt.Dir,
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
