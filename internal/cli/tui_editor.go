package cli

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/xrehpicx/wts/internal/config"
	"github.com/xrehpicx/wts/internal/model"
)

// --- Group editor ---

func (m *tuiModel) enterCreateGroupMode() tea.Cmd {
	m.createGroupMode = true
	m.createGroupFocus = createGroupFocusName
	m.createGroupCursor = 0
	m.createGroupSelected = make(map[string]bool, len(m.rc.project.Processes))
	m.createGroupInput.SetValue("")

	if target, ok := m.selectedTarget(); ok {
		for _, name := range target.ProcessNames {
			m.createGroupSelected[name] = true
		}
	}

	m.message = ""
	m.messageIsErr = false
	return m.createGroupInput.Focus()
}

func (m *tuiModel) updateCreateGroupKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "shift+tab" {
		msg = tea.KeyPressMsg{Code: tea.KeyTab}
	}
	switch msg.Code {
	case tea.KeyEsc:
		m.createGroupMode = false
		m.createGroupSelected = map[string]bool{}
		m.createGroupInput.Blur()
		return m, nil
	case tea.KeyTab:
		if m.createGroupFocus == createGroupFocusName {
			m.createGroupFocus = createGroupFocusMembers
			m.createGroupInput.Blur()
			return m, nil
		}
		m.createGroupFocus = createGroupFocusName
		return m, m.createGroupInput.Focus()
	case tea.KeyEnter:
		return m, m.saveCreateGroupCmd()
	}

	if m.createGroupFocus == createGroupFocusMembers {
		switch msg.Code {
		case tea.KeyUp, 'k':
			if len(m.rc.project.Processes) > 0 {
				m.createGroupCursor = (m.createGroupCursor - 1 + len(m.rc.project.Processes)) % len(m.rc.project.Processes)
			}
			return m, nil
		case tea.KeyDown, 'j':
			if len(m.rc.project.Processes) > 0 {
				m.createGroupCursor = (m.createGroupCursor + 1) % len(m.rc.project.Processes)
			}
			return m, nil
		case tea.KeySpace:
			processNames := m.rc.project.ProcessNames()
			if len(processNames) == 0 {
				return m, nil
			}
			name := processNames[m.createGroupCursor]
			if m.createGroupSelected[name] {
				delete(m.createGroupSelected, name)
			} else {
				m.createGroupSelected[name] = true
			}
			return m, nil
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.createGroupInput, cmd = m.createGroupInput.Update(msg)
	return m, cmd
}

func (m *tuiModel) selectedCreateGroupMembers() []string {
	members := make([]string, 0, len(m.createGroupSelected))
	for _, name := range m.rc.project.ProcessNames() {
		if m.createGroupSelected[name] {
			members = append(members, name)
		}
	}
	return members
}

func (m *tuiModel) saveCreateGroupCmd() tea.Cmd {
	if m.loading {
		return nil
	}
	name := strings.TrimSpace(m.createGroupInput.Value())
	members := m.selectedCreateGroupMembers()
	if name == "" {
		m.message = "group name is required"
		m.messageIsErr = true
		return nil
	}
	if len(members) == 0 {
		m.message = "select at least one process for the group"
		m.messageIsErr = true
		return nil
	}

	cfg := m.rc.project.Config()
	cfg.Groups = append(cfg.Groups, model.ProcessGroup{
		Name:      name,
		Processes: append([]string(nil), members...),
	})

	m.loading = true
	m.loadingMsg = "saving group " + name + "..."

	configPath := m.rc.project.ConfigPath
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		project, err := config.Save(configPath, cfg)
		if err != nil {
			return actionErrMsg{err: err}
		}
		target, err := project.ResolveTarget("", name)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return groupCreatedMsg{project: project, target: target}
	})
}

// --- Process filter ---

func (m *tuiModel) enterFilterMode() tea.Cmd {
	m.filterMode = true
	m.preFilterIdx = m.targetIdx
	m.filterInput.SetValue("")
	return m.filterInput.Focus()
}

func (m *tuiModel) updateFilterKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyDown, tea.KeyUp:
		delta := 1
		if msg.Code == tea.KeyUp {
			delta = -1
		}
		m.cycleFilterMatch(delta)
		return m, m.fetchLogsCmd()
	case tea.KeyEnter:
		if _, ok := m.selectedTarget(); !ok {
			m.targetIdx = m.preFilterIdx
		}
		m.filterMode = false
		m.filterInput.Blur()
		target, ok := m.selectedTarget()
		if ok {
			m.message = "target: " + formatTargetLabel(target)
		} else {
			m.message = ""
		}
		m.messageIsErr = false
		return m, m.fetchLogsCmd()
	case tea.KeyEscape:
		m.filterMode = false
		m.filterInput.Blur()
		m.targetIdx = m.preFilterIdx
		return m, m.fetchLogsCmd()
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterProcesses(m.filterInput.Value())
	return m, tea.Batch(cmd, m.fetchLogsCmd())
}

func (m *tuiModel) filterProcesses(query string) {
	if query == "" {
		m.targetIdx = m.preFilterIdx
		return
	}
	q := strings.ToLower(query)
	m.targetIdx = -1
	for i, target := range m.targets {
		if strings.Contains(strings.ToLower(formatTargetLabel(target)), q) {
			m.targetIdx = i
			return
		}
	}
}

func (m *tuiModel) cycleFilterMatch(delta int) {
	if len(m.targets) == 0 {
		return
	}
	q := strings.ToLower(m.filterInput.Value())
	for n := 1; n <= len(m.targets); n++ {
		i := ((m.targetIdx+delta*n)%len(m.targets) + len(m.targets)) % len(m.targets)
		if strings.Contains(strings.ToLower(formatTargetLabel(m.targets[i])), q) {
			m.targetIdx = i
			return
		}
	}
}
