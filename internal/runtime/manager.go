package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

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
	Attach(ctx context.Context, session, window string) error
}

type Manager struct {
	project *model.Project
	backend Backend
	session string
}

type SwitchOptions struct {
	Attach bool
}

type StatusRow struct {
	Workspace string `json:"workspace"`
	Group     string `json:"group"`
	Running   bool   `json:"running"`
	Active    bool   `json:"active"`
	Dir       string `json:"dir"`
}

func NewManager(project *model.Project, backend Backend) *Manager {
	return &Manager{
		project: project,
		backend: backend,
		session: tmux.SessionName(project.RootDir),
	}
}

func (m *Manager) Session() string {
	return m.session
}

func (m *Manager) ListWorkspaces() []model.Workspace {
	workspaces := make([]model.Workspace, len(m.project.Workspaces))
	copy(workspaces, m.project.Workspaces)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Name < workspaces[j].Name
	})
	return workspaces
}

func (m *Manager) Switch(ctx context.Context, workspace string, opts SwitchOptions) error {
	return m.activate(ctx, workspace, opts, false)
}

func (m *Manager) Start(ctx context.Context, workspace string, opts SwitchOptions) error {
	return m.activate(ctx, workspace, opts, false)
}

func (m *Manager) Restart(ctx context.Context, workspace string, opts SwitchOptions) error {
	return m.activate(ctx, workspace, opts, true)
}

func (m *Manager) activate(ctx context.Context, workspace string, opts SwitchOptions, forceRestart bool) error {
	ws, err := m.project.Workspace(workspace)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}

	activeKey := tmux.GroupOptionKey(ws.EffectiveGroup)
	activeWorkspace, err := m.backend.GetSessionOption(ctx, m.session, activeKey)
	if err != nil {
		return err
	}

	if activeWorkspace != "" && activeWorkspace != ws.Name {
		if prev, err := m.project.Workspace(activeWorkspace); err == nil {
			if err := m.stopWorkspaceProcess(ctx, prev); err != nil {
				return fmt.Errorf("stop previous workspace %q: %w", prev.Name, err)
			}
		}
	}

	if forceRestart {
		if err := m.stopWorkspaceProcess(ctx, ws); err != nil {
			return fmt.Errorf("restart workspace %q: %w", ws.Name, err)
		}
	}

	running, err := m.isRunning(ctx, ws)
	if err != nil {
		return err
	}
	if !running {
		if err := m.startWorkspaceProcess(ctx, ws); err != nil {
			return err
		}
	}

	if err := m.backend.SetSessionOption(ctx, m.session, activeKey, ws.Name); err != nil {
		return err
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey(), ws.Name); err != nil {
		return err
	}

	if opts.Attach {
		if err := m.backend.Attach(ctx, m.session, tmux.WindowName(ws.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) StopWorkspace(ctx context.Context, workspace string) error {
	ws, err := m.project.Workspace(workspace)
	if err != nil {
		return err
	}
	if err := m.ensureReady(ctx); err != nil {
		return err
	}

	if err := m.stopWorkspaceProcess(ctx, ws); err != nil {
		return err
	}

	activeKey := tmux.GroupOptionKey(ws.EffectiveGroup)
	active, err := m.backend.GetSessionOption(ctx, m.session, activeKey)
	if err != nil {
		return err
	}
	if active == ws.Name {
		if err := m.backend.SetSessionOption(ctx, m.session, activeKey, ""); err != nil {
			return err
		}
	}

	last, err := m.backend.GetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey())
	if err != nil {
		return err
	}
	if last == ws.Name {
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
	if !m.project.IsKnownWorkspace(active) {
		_ = m.backend.SetSessionOption(ctx, m.session, key, "")
		return nil
	}
	if err := m.StopWorkspace(ctx, active); err != nil {
		return err
	}
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}
	for i := range m.project.Workspaces {
		if err := m.stopWorkspaceProcess(ctx, &m.project.Workspaces[i]); err != nil {
			return err
		}
	}
	for _, group := range m.project.Groups() {
		if err := m.backend.SetSessionOption(ctx, m.session, tmux.GroupOptionKey(group), ""); err != nil {
			return err
		}
	}
	if err := m.backend.SetSessionOption(ctx, m.session, tmux.LastSelectedOptionKey(), ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Logs(ctx context.Context, workspace string, lines int) (string, error) {
	ws, err := m.project.Workspace(workspace)
	if err != nil {
		return "", err
	}
	if err := m.ensureReady(ctx); err != nil {
		return "", err
	}

	running, err := m.isRunning(ctx, ws)
	if err != nil {
		return "", err
	}
	if !running {
		return "", fmt.Errorf("workspace %q is not running", workspace)
	}

	return m.backend.CapturePane(ctx, m.session, tmux.WindowName(ws.Name), lines)
}

func (m *Manager) Status(ctx context.Context, workspace string) ([]StatusRow, error) {
	if err := m.ensureReady(ctx); err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0, len(m.project.Workspaces))
	for i := range m.project.Workspaces {
		ws := &m.project.Workspaces[i]
		if workspace != "" && workspace != ws.Name {
			continue
		}
		running, err := m.isRunning(ctx, ws)
		if err != nil {
			return nil, err
		}
		active, err := m.isActive(ctx, ws)
		if err != nil {
			return nil, err
		}
		rows = append(rows, StatusRow{
			Workspace: ws.Name,
			Group:     ws.EffectiveGroup,
			Running:   running,
			Active:    active,
			Dir:       ws.ResolvedDir,
		})
	}

	if workspace != "" && len(rows) == 0 {
		return nil, fmt.Errorf("workspace %q not found", workspace)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Workspace < rows[j].Workspace
	})
	return rows, nil
}

func (m *Manager) StatusJSON(ctx context.Context, workspace string) ([]byte, error) {
	rows, err := m.Status(ctx, workspace)
	if err != nil {
		return nil, err
	}
	if workspace != "" {
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

func (m *Manager) startWorkspaceProcess(ctx context.Context, ws *model.Workspace) error {
	window := tmux.WindowName(ws.Name)
	if err := m.backend.StartWindowCommand(
		ctx,
		m.session,
		window,
		ws.ResolvedDir,
		m.project.Defaults.Shell,
		ws.Command,
		ws.Env,
	); err != nil {
		return fmt.Errorf("start workspace %q: %w", ws.Name, err)
	}
	return nil
}

func (m *Manager) stopWorkspaceProcess(ctx context.Context, ws *model.Workspace) error {
	window := tmux.WindowName(ws.Name)
	timeout := time.Duration(m.project.Defaults.StopTimeoutSec) * time.Second
	if err := m.backend.StopWindow(ctx, m.session, window, timeout); err != nil {
		return fmt.Errorf("stop workspace %q: %w", ws.Name, err)
	}
	return nil
}

func (m *Manager) isRunning(ctx context.Context, ws *model.Workspace) (bool, error) {
	window := tmux.WindowName(ws.Name)
	return m.backend.HasWindow(ctx, m.session, window)
}

func (m *Manager) isActive(ctx context.Context, ws *model.Workspace) (bool, error) {
	active, err := m.backend.GetSessionOption(ctx, m.session, tmux.GroupOptionKey(ws.EffectiveGroup))
	if err != nil {
		return false, err
	}
	return active == ws.Name, nil
}
