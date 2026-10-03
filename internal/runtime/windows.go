package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/tmux"
)

// windowRef identifies a worktree window inside the tmux session that owns it.
// Every pane or window operation for a worktree must carry both parts: tmux
// resolves a window name without its session against the current session.
type windowRef struct {
	Session string
	Window  string
}

// locateWindows returns every tmux window that belongs to worktreeDir, the
// manager's own session first.
//
// The window name hashes the worktree path, so it identifies one worktree
// across all sessions. The session name does not: wts 0.4.0 and earlier named
// the session after the worktree that wts ran from, so a group started inside
// worktree A lives in A's session, while wts run from worktree B looked only in
// B's session and treated A's processes as stopped. Searching other wts
// sessions keeps those windows reachable for stop, status, logs, and attach.
func (m *Manager) locateWindows(ctx context.Context, worktreeDir string) ([]windowRef, error) {
	window := tmux.WindowName(worktreeDir)
	var refs []windowRef
	exists, err := m.backend.HasWindow(ctx, m.session, window)
	if err != nil {
		return nil, err
	}
	if exists {
		refs = append(refs, windowRef{Session: m.session, Window: window})
	}
	sessions, err := m.backend.FindWindowSessions(ctx, window)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session == m.session || !tmux.IsManagedSession(session) {
			continue
		}
		refs = append(refs, windowRef{Session: session, Window: window})
	}
	return refs, nil
}

// locateWindow returns the first window for worktreeDir. When none exists it
// returns a reference in the manager's session, where new windows are created.
func (m *Manager) locateWindow(ctx context.Context, worktreeDir string) (windowRef, bool, error) {
	own := windowRef{Session: m.session, Window: tmux.WindowName(worktreeDir)}
	exists, err := m.backend.HasWindow(ctx, own.Session, own.Window)
	if err != nil || exists {
		return own, exists, err
	}
	refs, err := m.locateWindows(ctx, worktreeDir)
	if err != nil {
		return windowRef{}, false, err
	}
	if len(refs) == 0 {
		return own, false, nil
	}
	return refs[0], true, nil
}

// listWorktreePanes returns the panes of every window that belongs to
// worktreeDir, paired with the window that owns each pane.
func (m *Manager) listWorktreePanes(ctx context.Context, worktreeDir string) ([]windowRef, [][]tmux.PaneInfo, error) {
	refs, err := m.locateWindows(ctx, worktreeDir)
	if err != nil {
		return nil, nil, err
	}
	panes := make([][]tmux.PaneInfo, 0, len(refs))
	for _, ref := range refs {
		items, err := m.backend.ListPanes(ctx, ref.Session, ref.Window)
		if err != nil {
			return nil, nil, err
		}
		panes = append(panes, items)
	}
	return refs, panes, nil
}

// foreignSessions returns the sessions in refs other than the manager's own.
func (m *Manager) foreignSessions(refs []windowRef) []string {
	var sessions []string
	for _, ref := range refs {
		if ref.Session != m.session && !stringInSlice(ref.Session, sessions) {
			sessions = append(sessions, ref.Session)
		}
	}
	return sessions
}

// clearActiveStateIn clears the active worktree options in session when they
// point at worktreeDir.
func (m *Manager) clearActiveStateIn(ctx context.Context, session, worktreeDir string) error {
	active, err := m.backend.GetSessionOption(ctx, session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return err
	}
	if active == "" || filepath.Clean(active) != filepath.Clean(worktreeDir) {
		return nil
	}
	return m.clearActiveOptions(ctx, session)
}

func (m *Manager) clearActiveOptions(ctx context.Context, session string) error {
	for _, key := range []string{
		tmux.ActiveWorktreeOptionKey(),
		tmux.ActiveProcessOptionKey(),
		tmux.ActiveTargetKindOptionKey(),
		tmux.ActiveTargetNameOptionKey(),
	} {
		if err := m.backend.SetSessionOption(ctx, session, key, ""); err != nil {
			return err
		}
	}
	return nil
}

// activeState is the active worktree bookkeeping stored on one tmux session.
type activeState struct {
	Dir       string
	Process   string
	Target    model.Target
	HasTarget bool
}

func (m *Manager) readActiveState(ctx context.Context, session string) (activeState, error) {
	dir, err := m.backend.GetSessionOption(ctx, session, tmux.ActiveWorktreeOptionKey())
	if err != nil {
		return activeState{}, err
	}
	if dir != "" {
		dir = filepath.Clean(dir)
	}
	proc, err := m.backend.GetSessionOption(ctx, session, tmux.ActiveProcessOptionKey())
	if err != nil {
		return activeState{}, err
	}
	target, ok := m.activeTargetIn(ctx, session)
	return activeState{Dir: dir, Process: proc, Target: target, HasTarget: ok}, nil
}

// paneProcessName returns the process identity recorded on a managed pane.
func paneProcessName(pane tmux.PaneInfo) string {
	if name := strings.TrimSpace(pane.Process); name != "" {
		return name
	}
	return tmux.ProcessFromPaneTitle(pane.Title)
}

func stopWorktreeError(worktreeDir string, err error) error {
	return fmt.Errorf("stop worktree %q: %w", worktreeDir, err)
}
