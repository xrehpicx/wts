package cli

import (
	"context"
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

// markedCmd runs fn in each marked worktree. Marked actions are additive: they
// never preempt the active worktree, so one target can run in many worktrees.
func (m *tuiModel) markedCmd(rows []runtime.StatusRow, progress, done string, fn func(context.Context, *runtime.Manager, string) error) tea.Cmd {
	selectors := make([]string, 0, len(rows))
	for _, row := range rows {
		selectors = append(selectors, row.Dir)
	}
	suffix := " in " + pluralWorktrees(len(rows))
	m.startLoading(progress+suffix+"...", selectors...)
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		err := manager.ForEachWorktree(ctx, selectors, func(ctx context.Context, wt gitwt.Worktree) error {
			return fn(ctx, manager, wt.Dir)
		})
		if err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: done + suffix}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func pluralWorktrees(n int) string {
	if n == 1 {
		return "1 worktree"
	}
	return fmt.Sprintf("%d worktrees", n)
}

func (m *tuiModel) requireTarget() (model.Target, bool) {
	target, ok := m.selectedTarget()
	if !ok {
		m.message = "select a process or group first with ←/→"
		m.messageIsErr = true
	}
	return target, ok
}

func (m *tuiModel) switchCurrentCmd() tea.Cmd {
	if marked := m.markedRows(); len(marked) > 0 {
		target, ok := m.requireTarget()
		if !ok {
			return nil
		}
		label := formatTargetLabel(target)
		return m.markedCmd(marked, "starting "+label, "started "+label, func(ctx context.Context, manager *runtime.Manager, dir string) error {
			return manager.Start(ctx, dir, runOptionsForTarget(target))
		})
	}
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.requireTarget()
	if !ok {
		return nil
	}
	dir, name := row.Dir, row.Worktree

	// If the worktree has at least one live process, use Start (additive).
	// Otherwise use Switch (preemptive: stops the other active worktree).
	useAdditive := row.Running && !row.Exited
	if useAdditive {
		m.startLoading("starting "+formatTargetLabel(target)+" in "+name+"...", dir)
	} else {
		m.startLoading("switching "+formatTargetLabel(target)+" to "+name+"...", dir)
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
	if marked := m.markedRows(); len(marked) > 0 {
		target, ok := m.requireTarget()
		if !ok {
			return nil
		}
		label := formatTargetLabel(target)
		return m.markedCmd(marked, "restarting "+label, "restarted "+label, func(ctx context.Context, manager *runtime.Manager, dir string) error {
			return manager.Restart(ctx, dir, runOptionsForTarget(target))
		})
	}
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.requireTarget()
	if !ok {
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.startLoading("restarting "+formatTargetLabel(target)+" in "+name+"...", dir)
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		if err := manager.Restart(ctx, dir, runOptionsForTarget(target)); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "restarted " + formatTargetLabel(target) + " in " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func stopTarget(ctx context.Context, manager *runtime.Manager, dir string, target model.Target) error {
	if target.Kind == model.TargetGroup {
		return manager.StopGroup(ctx, dir, target.Name)
	}
	return manager.StopProcess(ctx, dir, target.Name)
}

func (m *tuiModel) stopCurrentCmd() tea.Cmd {
	if marked := m.markedRows(); len(marked) > 0 {
		target, ok := m.requireTarget()
		if !ok {
			return nil
		}
		label := formatTargetLabel(target)
		running := marked[:0:0]
		for i := range marked {
			if managed, _ := targetProcessState(&marked[i], target); managed {
				running = append(running, marked[i])
			}
		}
		if len(running) == 0 {
			m.message = label + " is not running in any marked worktree"
			m.messageIsErr = true
			return nil
		}
		return m.markedCmd(running, "stopping "+label, "stopped "+label, func(ctx context.Context, manager *runtime.Manager, dir string) error {
			return stopTarget(ctx, manager, dir, target)
		})
	}
	row := m.current()
	if row == nil {
		return nil
	}
	target, ok := m.requireTarget()
	if !ok {
		return nil
	}
	if managed, _ := targetProcessState(row, target); !managed {
		m.message = formatTargetLabel(target) + " is not running in " + row.Worktree
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.startLoading("stopping "+formatTargetLabel(target)+" in "+name+"...", dir)
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		if err := stopTarget(ctx, manager, dir, target); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "stopped " + formatTargetLabel(target) + " in " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) stopAllCurrentCmd() tea.Cmd {
	if marked := m.markedRows(); len(marked) > 0 {
		return m.markedCmd(marked, "stopping everything", "stopped everything", func(ctx context.Context, manager *runtime.Manager, dir string) error {
			_, err := manager.StopWorktree(ctx, dir)
			return err
		})
	}
	row := m.current()
	if row == nil {
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.startLoading("stopping all in "+name+"...", dir)
	manager, ctx := m.rc.manager, m.rc.context()
	action := func() tea.Msg {
		if _, err := manager.StopWorktree(ctx, dir); err != nil {
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
	target, ok := m.requireTarget()
	if !ok {
		return nil
	}
	if managed, _ := targetProcessState(row, target); !managed {
		m.message = formatTargetLabel(target) + " is not running in " + row.Worktree
		m.messageIsErr = true
		return nil
	}

	dir, name := row.Dir, row.Worktree
	m.startLoading("attaching "+formatTargetLabel(target)+" in "+name+"...", dir)

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
