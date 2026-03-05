package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/state"
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
	Attach(ctx context.Context, session, window string) error
}

type Manager struct {
	project *model.Project
	repo    *state.RepoState
	backend Backend
	session string
}

type SwitchOptions struct {
	Attach bool
}

type StatusRow struct {
	Worktree string `json:"worktree"`
	Process  string `json:"process"`
	Group    string `json:"group"`
	Running  bool   `json:"running"`
	Active   bool   `json:"active"`
	Dir      string `json:"dir"`
}

func NewManager(project *model.Project, repo *state.RepoState, backend Backend) *Manager {
	return &Manager{
		project: project,
		repo:    repo,
		backend: backend,
		session: tmux.SessionName(repo.Root),
	}
}

func (m *Manager) Session() string {
	return m.session
}

func (m *Manager) ListWorktrees() []state.Worktree {
	items := make([]state.Worktree, len(m.repo.Worktrees))
	copy(items, m.repo.Worktrees)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (m *Manager) Switch(ctx context.Context, worktree string, opts SwitchOptions) error {
	return m.activate(ctx, worktree, opts, false)
}

func (m *Manager) Start(ctx context.Context, worktree string, opts SwitchOptions) error {
	return m.activate(ctx, worktree, opts, false)
}

func (m *Manager) Restart(ctx context.Context, worktree string, opts SwitchOptions) error {
	return m.activate(ctx, worktree, opts, true)
}

func (m *Manager) activate(ctx context.Context, worktree string, opts SwitchOptions, forceRestart bool) error {
	wt, proc, group, err := m.resolveWorktree(worktree)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}

	activeKey := tmux.GroupOptionKey(group)
	activeWorktree, err := m.backend.GetSessionOption(ctx, m.session, activeKey)
	if err != nil {
		return err
	}
	if activeWorktree != "" && activeWorktree != wt.Name {
		if prev, _, _, prevErr := m.resolveWorktree(activeWorktree); prevErr == nil {
			if err := m.stopWorktreeProcess(ctx, prev); err != nil {
				return fmt.Errorf("stop previous worktree %q: %w", prev.Name, err)
			}
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
	if !running {
		if err := m.startWorktreeProcess(ctx, wt, proc); err != nil {
			return err
		}
	}

	if err := m.backend.SetSessionOption(ctx, m.session, activeKey, wt.Name); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey(), wt.Name); err != nil {
		return err
	}

	if opts.Attach {
		if err := m.backend.Attach(ctx, m.session, tmux.WindowName(wt.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) StopWorktree(ctx context.Context, worktree string) error {
	wt, _, group, err := m.resolveWorktree(worktree)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	if err := m.stopWorktreeProcess(ctx, wt); err != nil {
		return err
	}

	activeKey := tmux.GroupOptionKey(group)
	active, err := m.backend.GetSessionOption(ctx, m.session, activeKey)
	if err != nil {
		return err
	}
	if active == wt.Name {
		if err := m.backend.SetSessionOption(ctx, m.session, activeKey, ""); err != nil {
			return err
		}
	}

	last, err := m.backend.GetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey())
	if err != nil {
		return err
	}
	if last == wt.Name {
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey(), ""); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) StopGroup(ctx context.Context, group string) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	key := tmux.GroupOptionKey(group)
	active, err := m.backend.GetSessionOption(ctx, m.session, key)
	if err != nil {
		return err
	}
	if active == "" {
		return nil
	}
	if _, idx := m.repo.Worktree(active); idx < 0 {
		_ = m.backend.SetSessionOption(ctx, m.session, key, "")
		return nil
	}
	return m.StopWorktree(ctx, active)
}

func (m *Manager) StopAll(ctx context.Context) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	for i := range m.repo.Worktrees {
		if err := m.stopWorktreeProcess(ctx, &m.repo.Worktrees[i]); err != nil {
			return err
		}
	}
	groups := map[string]struct{}{}
	for i := range m.repo.Worktrees {
		_, proc, group, err := m.resolveWorktree(m.repo.Worktrees[i].Name)
		if err != nil || proc == nil {
			continue
		}
		groups[group] = struct{}{}
	}
	for g := range groups {
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.GroupOptionKey(g), ""); err != nil {
			return err
		}
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey(), ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Logs(ctx context.Context, worktree string, lines int) (string, error) {
	wt, _, _, err := m.resolveWorktree(worktree)
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
		return "", fmt.Errorf("worktree %q is not running", worktree)
	}
	return m.backend.CapturePane(ctx, m.session, tmux.WindowName(wt.Name), lines)
}

func (m *Manager) Status(ctx context.Context, worktree string) ([]StatusRow, error) {
	if err := m.ensureReady(ctx); err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0, len(m.repo.Worktrees))
	for i := range m.repo.Worktrees {
		wt := &m.repo.Worktrees[i]
		if worktree != "" && wt.Name != worktree {
			continue
		}
		proc, err := m.project.Process(wt.Process)
		if err != nil {
			return nil, err
		}
		group := model.EffectiveGroup(proc, wt.Group, wt.Name)
		running, err := m.isRunning(ctx, wt)
		if err != nil {
			return nil, err
		}
		active, err := m.isActive(ctx, wt, group)
		if err != nil {
			return nil, err
		}
		rows = append(rows, StatusRow{
			Worktree: wt.Name,
			Process:  wt.Process,
			Group:    group,
			Running:  running,
			Active:   active,
			Dir:      wt.Dir,
		})
	}

	if worktree != "" && len(rows) == 0 {
		return nil, fmt.Errorf("worktree %q not found", worktree)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Worktree < rows[j].Worktree })
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

func (m *Manager) ensureReady(ctx context.Context) error {
	if err := m.backend.EnsureTmux(ctx); err != nil {
		return err
	}
	if err := m.backend.EnsureSession(ctx, m.session); err != nil {
		return err
	}
	return nil
}

func (m *Manager) resolveWorktree(name string) (*state.Worktree, *model.Process, string, error) {
	wt, _ := m.repo.Worktree(name)
	if wt == nil {
		return nil, nil, "", fmt.Errorf("worktree %q not found", name)
	}
	proc, err := m.project.Process(wt.Process)
	if err != nil {
		return nil, nil, "", err
	}
	group := model.EffectiveGroup(proc, wt.Group, wt.Name)
	return wt, proc, group, nil
}

func (m *Manager) startWorktreeProcess(ctx context.Context, wt *state.Worktree, proc *model.Process) error {
	window := tmux.WindowName(wt.Name)
	if err := m.backend.StartWindowCommand(
		ctx,
		m.session,
		window,
		wt.Dir,
		m.project.Defaults.Shell,
		proc.Command,
		proc.Env,
	); err != nil {
		return fmt.Errorf("start worktree %q: %w", wt.Name, err)
	}
	return nil
}

func (m *Manager) stopWorktreeProcess(ctx context.Context, wt *state.Worktree) error {
	timeout := time.Duration(m.project.Defaults.StopTimeoutSec) * time.Second
	if err := m.backend.StopWindow(ctx, m.session, tmux.WindowName(wt.Name), timeout); err != nil {
		return fmt.Errorf("stop worktree %q: %w", wt.Name, err)
	}
	return nil
}

func (m *Manager) isRunning(ctx context.Context, wt *state.Worktree) (bool, error) {
	return m.backend.HasWindow(ctx, m.session, tmux.WindowName(wt.Name))
}

func (m *Manager) isActive(ctx context.Context, wt *state.Worktree, group string) (bool, error) {
	active, err := m.backend.GetSessionOption(ctx, m.session, tmux.GroupOptionKey(group))
	if err != nil {
		return false, err
	}
	return active == wt.Name, nil
}
