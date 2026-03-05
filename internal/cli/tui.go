package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/state"
)

type tuiModel struct {
	rc      *runtimeContext
	idx     int
	message string
	rows    []runtime.StatusRow
}

func newTUIModel(rc *runtimeContext) *tuiModel {
	idx := rc.repo.Selected
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rc.repo.Worktrees) {
		idx = max(0, len(rc.repo.Worktrees)-1)
	}
	m := &tuiModel{rc: rc, idx: idx}
	m.refreshStatus()
	return m
}

func (m *tuiModel) Init() tea.Cmd { return nil }

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "j", "down", "l", "right", "n":
			m.next()
		case "k", "up", "h", "left", "p":
			m.prev()
		case "enter", "s":
			m.switchCurrent()
		case "r":
			m.restartCurrent()
		case "x":
			m.stopCurrent()
		case "a":
			m.addCurrentDir()
		case "d":
			m.removeCurrent()
		case "g":
			m.applyProcessGroup()
		case "u":
			m.clearGroupOverride()
		case "]":
			m.cycleProcess(1)
		case "[":
			m.cycleProcess(-1)
		}
	}
	return m, nil
}

func (m *tuiModel) View() string {
	var b strings.Builder
	b.WriteString("workswitch TUI (wts = worktree switch)\n")
	b.WriteString(fmt.Sprintf("Repo: %s\n", m.rc.repo.Root))
	b.WriteString(fmt.Sprintf("State: %s\n\n", m.rc.store.Path))

	if len(m.rc.repo.Worktrees) == 0 {
		b.WriteString("No worktrees configured. Press 'a' to add current directory.\n")
		b.WriteString("\nKeys: a add-current-dir | q quit\n")
		if m.message != "" {
			b.WriteString("\n" + m.message + "\n")
		}
		return b.String()
	}

	for i, wt := range m.rc.repo.Worktrees {
		prefix := "  "
		if i == m.idx {
			prefix = "> "
		}
		status := "stopped"
		for _, row := range m.rows {
			if row.Worktree == wt.Name {
				if row.Running {
					status = "running"
				}
				if row.Active {
					status += ",active"
				}
				break
			}
		}
		group := wt.Group
		if proc, err := m.rc.project.Process(wt.Process); err == nil {
			group = model.EffectiveGroup(proc, wt.Group, wt.Name)
		}
		b.WriteString(fmt.Sprintf("%s%s  proc=%s  group=%s  %s\n", prefix, wt.Name, wt.Process, group, status))
		b.WriteString(fmt.Sprintf("    dir: %s\n", wt.Dir))
	}

	b.WriteString("\nKeys: n/p next/prev  s enter switch  r restart  x stop\n")
	b.WriteString("      a add-cwd  d remove  [ ] change-process  g use-process-group  u clear-group\n")
	b.WriteString("      q quit\n")
	if m.message != "" {
		b.WriteString("\n" + m.message + "\n")
	}
	return b.String()
}

func (m *tuiModel) next() {
	if len(m.rc.repo.Worktrees) == 0 {
		return
	}
	m.idx = (m.idx + 1) % len(m.rc.repo.Worktrees)
	m.rc.repo.Selected = m.idx
	_ = m.rc.store.Save(m.rc.file)
}

func (m *tuiModel) prev() {
	if len(m.rc.repo.Worktrees) == 0 {
		return
	}
	m.idx = (m.idx - 1 + len(m.rc.repo.Worktrees)) % len(m.rc.repo.Worktrees)
	m.rc.repo.Selected = m.idx
	_ = m.rc.store.Save(m.rc.file)
}

func (m *tuiModel) current() *state.Worktree {
	if len(m.rc.repo.Worktrees) == 0 || m.idx < 0 || m.idx >= len(m.rc.repo.Worktrees) {
		return nil
	}
	return &m.rc.repo.Worktrees[m.idx]
}

func (m *tuiModel) switchCurrent() {
	wt := m.current()
	if wt == nil {
		return
	}
	if err := m.rc.manager.Switch(context.Background(), wt.Name, runtime.SwitchOptions{}); err != nil {
		m.message = err.Error()
		return
	}
	m.rc.repo.Selected = m.idx
	_ = m.rc.store.Save(m.rc.file)
	m.message = "switched to " + wt.Name
	m.refreshStatus()
}

func (m *tuiModel) restartCurrent() {
	wt := m.current()
	if wt == nil {
		return
	}
	if err := m.rc.manager.Restart(context.Background(), wt.Name, runtime.SwitchOptions{}); err != nil {
		m.message = err.Error()
		return
	}
	m.message = "restarted " + wt.Name
	m.refreshStatus()
}

func (m *tuiModel) stopCurrent() {
	wt := m.current()
	if wt == nil {
		return
	}
	if err := m.rc.manager.StopWorktree(context.Background(), wt.Name); err != nil {
		m.message = err.Error()
		return
	}
	m.message = "stopped " + wt.Name
	m.refreshStatus()
}

func (m *tuiModel) addCurrentDir() {
	cwd := m.rc.repo.Root
	name := autoWorktreeName(cwd, m.rc.repo)
	procNames := m.rc.project.ProcessNames()
	if len(procNames) == 0 {
		m.message = "no processes configured"
		return
	}
	m.rc.repo.Worktrees = append(m.rc.repo.Worktrees, state.Worktree{
		Name:    name,
		Dir:     cwd,
		Process: procNames[0],
	})
	_ = m.rc.repo.Normalize()
	m.idx, _ = indexOfWorktree(m.rc.repo, name)
	m.rc.repo.Selected = m.idx
	if err := m.rc.store.Save(m.rc.file); err != nil {
		m.message = err.Error()
		return
	}
	m.message = "added " + name + " from " + filepath.Base(cwd)
	m.refreshStatus()
}

func (m *tuiModel) removeCurrent() {
	wt := m.current()
	if wt == nil {
		return
	}
	name := wt.Name
	_ = m.rc.repo.RemoveWorktree(name)
	if len(m.rc.repo.Worktrees) == 0 {
		m.idx = 0
	} else if m.idx >= len(m.rc.repo.Worktrees) {
		m.idx = len(m.rc.repo.Worktrees) - 1
	}
	m.rc.repo.Selected = m.idx
	if err := m.rc.store.Save(m.rc.file); err != nil {
		m.message = err.Error()
		return
	}
	m.message = "removed " + name
	m.refreshStatus()
}

func (m *tuiModel) cycleProcess(delta int) {
	wt := m.current()
	if wt == nil {
		return
	}
	names := m.rc.project.ProcessNames()
	if len(names) == 0 {
		m.message = "no processes configured"
		return
	}
	cur := 0
	for i := range names {
		if names[i] == wt.Process {
			cur = i
			break
		}
	}
	next := (cur + delta + len(names)) % len(names)
	wt.Process = names[next]
	if err := m.rc.store.Save(m.rc.file); err != nil {
		m.message = err.Error()
		return
	}
	m.message = fmt.Sprintf("assigned %s -> %s", wt.Name, wt.Process)
	m.refreshStatus()
}

func (m *tuiModel) applyProcessGroup() {
	wt := m.current()
	if wt == nil {
		return
	}
	proc, err := m.rc.project.Process(wt.Process)
	if err != nil {
		m.message = err.Error()
		return
	}
	if proc.Group == "" {
		m.message = "selected process has no group"
		return
	}
	wt.Group = proc.Group
	if err := m.rc.store.Save(m.rc.file); err != nil {
		m.message = err.Error()
		return
	}
	m.message = fmt.Sprintf("group override set: %s -> %s", wt.Name, wt.Group)
	m.refreshStatus()
}

func (m *tuiModel) clearGroupOverride() {
	wt := m.current()
	if wt == nil {
		return
	}
	wt.Group = ""
	if err := m.rc.store.Save(m.rc.file); err != nil {
		m.message = err.Error()
		return
	}
	m.message = fmt.Sprintf("group override cleared: %s", wt.Name)
	m.refreshStatus()
}

func (m *tuiModel) refreshStatus() {
	rows, err := m.rc.manager.Status(context.Background(), "")
	if err != nil {
		m.rows = nil
		m.message = err.Error()
		return
	}
	m.rows = rows
}

func indexOfWorktree(repo *state.RepoState, name string) (int, bool) {
	for i := range repo.Worktrees {
		if repo.Worktrees[i].Name == name {
			return i, true
		}
	}
	return 0, false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
