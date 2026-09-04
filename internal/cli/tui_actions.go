package cli

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

// --- Async actions ---

func (m *tuiModel) switchCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.selectedTarget()
	if !ok {
		m.message = "select a process or group first with ←/→"
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir

	// If the worktree has at least one live process, use Start (additive).
	// Otherwise use Switch (preemptive: stops the other active worktree).
	useAdditive := row.Running && !row.Exited
	if useAdditive {
		m.loadingMsg = "starting " + formatTargetLabel(target) + " in " + name + "..."
	} else {
		m.loadingMsg = "switching " + formatTargetLabel(target) + " to " + name + "..."
	}

	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		opts := runOptionsForTarget(target)
		var err error
		if useAdditive {
			err = manager.Start(ctx, dir, opts)
		} else {
			err = manager.Switch(ctx, dir, opts)
		}
		if err != nil {
			return actionErrMsg{err: err}
		}
		if useAdditive {
			return actionDoneMsg{text: "started " + formatTargetLabel(target) + " in " + name}
		}
		return actionDoneMsg{text: "switched " + formatTargetLabel(target) + " to " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) restartCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.selectedTarget()
	if !ok {
		m.message = "select a process or group first with ←/→"
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "restarting " + formatTargetLabel(target) + " in " + name + "..."
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		if err := manager.Restart(ctx, dir, runOptionsForTarget(target)); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "restarted " + formatTargetLabel(target) + " in " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) stopCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.selectedTarget()
	if !ok {
		m.message = "select a process or group first with ←/→"
		m.messageIsErr = true
		return nil
	}
	if managed, _ := targetProcessState(row, target); !managed {
		m.message = formatTargetLabel(target) + " is not running in " + row.Worktree
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "stopping " + formatTargetLabel(target) + " in " + name + "..."
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		var err error
		if target.Kind == model.TargetGroup {
			err = manager.StopGroup(ctx, dir, target.Name)
		} else {
			err = manager.StopProcess(ctx, dir, target.Name)
		}
		if err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "stopped " + formatTargetLabel(target) + " in " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) stopAllCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "stopping all in " + name + "..."
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		if err := manager.StopWorktree(ctx, dir); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "stopped all in " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) attachCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.selectedTarget()
	if !ok {
		m.message = "select a process or group first with ←/→"
		m.messageIsErr = true
		return nil
	}
	if managed, _ := targetProcessState(row, target); !managed {
		m.message = formatTargetLabel(target) + " is not running in " + row.Worktree
		m.messageIsErr = true
		return nil
	}

	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "attaching " + formatTargetLabel(target) + " in " + name + "..."

	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		spec, err := manager.ResolveAttach(ctx, dir, runOptionsForTarget(target))
		if err != nil {
			return actionErrMsg{err: err}
		}
		return attachReadyMsg{spec: spec}
	}
	return tea.Batch(m.spinner.Tick, action)
}

// --- Log streaming ---

func (m *tuiModel) fetchLogsCmd() tea.Cmd {
	m.logRequest++
	m.logPending = false
	request := m.logRequest
	row := m.current()
	if row == nil {
		return nil
	}
	dir := row.Dir
	target, ok := m.selectedTarget()
	if !ok {
		m.logLines = nil
		return nil
	}
	manager, ctx := m.rc.manager, m.rc.context()
	m.logPending = true
	return func() tea.Msg {
		linesByTarget := map[string][]string{}
		if target.Kind == model.TargetGroup {
			for _, processName := range target.ProcessNames {
				output, err := manager.Logs(ctx, dir, processName, 200)
				if err != nil {
					continue
				}
				raw := strings.TrimRight(output, "\n")
				if raw == "" {
					continue
				}
				linesByTarget[processName] = strings.Split(raw, "\n")
			}
			return logsMsg{request: request, dir: dir, linesByTarget: linesByTarget}
		}

		processName := target.Name
		output, err := manager.Logs(ctx, dir, processName, 200)
		if err != nil {
			return logsMsg{request: request, dir: dir, linesByTarget: linesByTarget}
		}
		raw := strings.TrimRight(output, "\n")
		if raw == "" {
			return logsMsg{request: request, dir: dir, linesByTarget: linesByTarget}
		}
		linesByTarget[processName] = strings.Split(raw, "\n")
		return logsMsg{request: request, dir: dir, linesByTarget: linesByTarget}
	}
}

func (m *tuiModel) scheduleLogRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tickLogsMsg{}
	})
}

// --- Status refresh ---

func (m *tuiModel) refreshStatusCmd() tea.Cmd {
	m.statusRequest++
	m.statusPending = true
	request := m.statusRequest
	manager, ctx, repoRoot := m.rc.manager, m.rc.context(), m.rc.repoRoot
	return func() tea.Msg {
		wts, err := gitwt.DiscoverContext(ctx, repoRoot)
		if err != nil {
			return statusRefreshedMsg{request: request, err: err}
		}
		rows, err := manager.StatusForWorktrees(ctx, wts)
		return statusRefreshedMsg{request: request, rows: rows, worktrees: wts, err: err}
	}
}

func (m *tuiModel) refreshStatus() {
	currentDir := ""
	if cur := m.current(); cur != nil {
		currentDir = cur.Dir
	}

	rows, err := m.rc.manager.Status(m.rc.context(), "")
	if err != nil {
		m.rows = nil
		m.message = err.Error()
		m.messageIsErr = true
		return
	}
	m.rows = rows
	if len(rows) == 0 {
		m.idx = 0
		return
	}
	for i := range rows {
		if rows[i].Dir == currentDir {
			m.idx = i
			return
		}
	}
	if m.idx >= len(rows) {
		m.idx = len(rows) - 1
	}
}

// --- Quit info ---

func (m *tuiModel) buildQuitInfo() {
	var running []runtime.StatusRow
	for _, row := range m.rows {
		if row.Running {
			running = append(running, row)
		}
	}
	if len(running) == 0 {
		return
	}

	session := m.rc.manager.Session()
	var b strings.Builder
	b.WriteString("\n  Processes still running in session \"" + session + "\":\n\n")
	for _, row := range running {
		window := tmux.WindowName(row.Dir)
		mark := "●"
		if row.Active {
			mark = "★"
		}
		_, _ = fmt.Fprintf(&b, "    %s %s [%s]\n", mark, row.Worktree, row.Branch)
		_, _ = fmt.Fprintf(&b, "      tmux attach -t %s \\; select-window -t %s:%s\n\n", session, session, window)
	}
	m.quitInfo = b.String()
}
