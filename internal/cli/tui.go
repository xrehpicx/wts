package cli

import (
	"image/color"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
)

type actionDoneMsg struct{ text string }
type actionErrMsg struct{ err error }
type attachReadyMsg struct{ spec runtime.AttachSpec }
type groupCreatedMsg struct {
	project *model.Project
	target  model.Target
}
type statusRefreshedMsg struct {
	request   uint64
	rows      []runtime.StatusRow
	worktrees []gitwt.Worktree
	err       error
}
type logsMsg struct {
	request       uint64
	dir           string
	linesByTarget map[string][]string
}
type tickLogsMsg struct{}

type tuiStyles struct {
	title       lipgloss.Style
	subtitle    lipgloss.Style
	statusOk    lipgloss.Style
	statusErr   lipgloss.Style
	statusBusy  lipgloss.Style
	panelTitle  lipgloss.Style
	panelBorder lipgloss.Style
	panelFocus  lipgloss.Style
	selectedRow lipgloss.Style
	row         lipgloss.Style
	colHeader   lipgloss.Style
	runDot      lipgloss.Style
	exitedDot   lipgloss.Style
	stopDot     lipgloss.Style
	activeMark  lipgloss.Style
	metaLabel   lipgloss.Style
	metaValue   lipgloss.Style
	separator   lipgloss.Style
	dimText     lipgloss.Style
	logText     lipgloss.Style
	footer      lipgloss.Style
	modalBorder lipgloss.Style
	modalFocus  lipgloss.Style
}

type createGroupFocus int

const (
	createGroupFocusName createGroupFocus = iota
	createGroupFocusMembers
)

type tuiModel struct {
	statusPending       bool
	logPending          bool
	statusRequest       uint64
	logRequest          uint64
	rc                  *runtimeContext
	idx                 int
	listOffset          int
	width               int
	height              int
	keys                tuiKeyMap
	showAll             bool
	message             string
	messageIsErr        bool
	rows                []runtime.StatusRow
	targets             []model.Target
	targetIdx           int
	styles              tuiStyles
	spinner             spinner.Model
	loading             bool
	loadingDir          string
	loadingMsg          string
	quitInfo            string
	attachSpec          *runtime.AttachSpec
	filterMode          bool
	filterInput         textinput.Model
	preFilterIdx        int
	logLines            map[string][]string
	logDir              string
	createGroupMode     bool
	createGroupInput    textinput.Model
	createGroupFocus    createGroupFocus
	createGroupCursor   int
	createGroupSelected map[string]bool
}

type tuiKeyMap struct {
	Next        key.Binding
	Prev        key.Binding
	Switch      key.Binding
	Restart     key.Binding
	Stop        key.Binding
	StopAll     key.Binding
	Attach      key.Binding
	ProcPrev    key.Binding
	ProcNext    key.Binding
	Filter      key.Binding
	CreateGroup key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func newTUIKeyMap() tuiKeyMap {
	return tuiKeyMap{
		Next:        key.NewBinding(key.WithKeys("n", "j", "down"), key.WithHelp("j/↓", "next")),
		Prev:        key.NewBinding(key.WithKeys("p", "k", "up"), key.WithHelp("k/↑", "prev")),
		Switch:      key.NewBinding(key.WithKeys("s", "enter"), key.WithHelp("s/↵", "start/switch")),
		Restart:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart target")),
		Stop:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop target")),
		StopAll:     key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "stop all")),
		Attach:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "attach tmux")),
		ProcPrev:    key.NewBinding(key.WithKeys("h", "left", "["), key.WithHelp("h/←", "prev target")),
		ProcNext:    key.NewBinding(key.WithKeys("l", "right", "]"), key.WithHelp("l/→", "next target")),
		Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search target")),
		CreateGroup: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "new group")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev, k.Switch, k.Restart, k.Stop, k.StopAll, k.Attach},
		{k.ProcPrev, k.ProcNext, k.Filter, k.CreateGroup, k.Help, k.Quit},
	}
}

func newTUIStyles(isDark bool) tuiStyles {
	lightDark := lipgloss.LightDark(isDark)
	ac := func(light, dark string) color.Color {
		return lightDark(lipgloss.Color(light), lipgloss.Color(dark))
	}
	return tuiStyles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ac("#0B3954", "#D9ECFF")),
		subtitle: lipgloss.NewStyle().
			Foreground(ac("#4A5568", "#9AAABD")),
		statusOk: lipgloss.NewStyle().
			Foreground(ac("#166534", "#8FE3B2")),
		statusErr: lipgloss.NewStyle().
			Foreground(ac("#B91C1C", "#FF9C9C")).
			Bold(true),
		statusBusy: lipgloss.NewStyle().
			Foreground(ac("#92400E", "#FFD392")),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ac("#1D4E89", "#A9CAFF")),
		panelBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ac("#B8C5D6", "#3A4E68")).
			Padding(0, 1),
		panelFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ac("#5A84B5", "#7DB2FF")).
			Padding(0, 1),
		selectedRow: lipgloss.NewStyle().
			Foreground(ac("#0B2239", "#F4F9FF")).
			Background(ac("#D9EBFF", "#25384E")).
			Bold(true),
		row:       lipgloss.NewStyle().Foreground(ac("#1F2937", "#D0DBE8")),
		colHeader: lipgloss.NewStyle().Foreground(ac("#3E5C76", "#98ADC7")).Bold(true),
		runDot:    lipgloss.NewStyle().Foreground(ac("#166534", "#8FE3B2")),
		exitedDot: lipgloss.NewStyle().Foreground(ac("#92400E", "#FFD392")),
		stopDot:   lipgloss.NewStyle().Foreground(ac("#6B7280", "#8796A8")),
		activeMark: lipgloss.NewStyle().
			Foreground(ac("#92400E", "#FFD392")).
			Bold(true),
		metaLabel: lipgloss.NewStyle().
			Foreground(ac("#46607A", "#98ADC7")).
			Width(10),
		metaValue: lipgloss.NewStyle().
			Foreground(ac("#0F172A", "#E6EEF7")),
		separator: lipgloss.NewStyle().
			Foreground(ac("#D1D5DB", "#2D3F54")),
		dimText: lipgloss.NewStyle().
			Foreground(ac("#6B7280", "#8796A8")),
		logText: lipgloss.NewStyle().
			Foreground(ac("#374151", "#9CAABB")),
		footer: lipgloss.NewStyle().
			Foreground(ac("#4B5563", "#9AAABD")),
		modalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ac("#7A8FA8", "#55759A")).
			Padding(0, 1),
		modalFocus: lipgloss.NewStyle().
			Foreground(ac("#0B2239", "#F4F9FF")).
			Background(ac("#D9EBFF", "#25384E")).
			Bold(true),
	}
}

func (m *tuiModel) applyColorScheme(isDark bool) {
	m.styles = newTUIStyles(isDark)
	m.filterInput.SetStyles(textinput.DefaultStyles(isDark))
	m.createGroupInput.SetStyles(textinput.DefaultStyles(isDark))
	lightDark := lipgloss.LightDark(isDark)
	m.spinner.Style = lipgloss.NewStyle().Foreground(
		lightDark(lipgloss.Color("#92400E"), lipgloss.Color("#FFD392")),
	)
}

func newTUIModel(rc *runtimeContext) *tuiModel {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"◒", "◐", "◓", "◑"},
		FPS:    80 * time.Millisecond,
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64

	createGroupInput := textinput.New()
	createGroupInput.Prompt = ""
	createGroupInput.CharLimit = 64

	m := &tuiModel{
		rc:                  rc,
		keys:                newTUIKeyMap(),
		styles:              newTUIStyles(true),
		spinner:             s,
		filterInput:         ti,
		targetIdx:           0,
		createGroupInput:    createGroupInput,
		createGroupSelected: map[string]bool{},
	}
	m.applyColorScheme(true)

	targets := rc.project.Targets()
	if activeTarget, ok := rc.manager.ActiveTarget(rc.context()); ok {
		reordered := make([]model.Target, 0, len(targets))
		reordered = append(reordered, activeTarget)
		for _, target := range targets {
			if !sameTarget(target, activeTarget) {
				reordered = append(reordered, target)
			}
		}
		m.targets = reordered
	} else {
		m.targets = targets
	}

	m.refreshStatus()
	if len(m.rows) > 0 {
		for i := range m.rows {
			if m.rows[i].Active {
				m.idx = i
				if activeTarget, ok := rc.manager.ActiveTarget(rc.context()); ok {
					m.selectTarget(activeTarget)
				}
				break
			}
		}
	}
	return m
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(m.fetchLogsCmd(), m.scheduleLogRefresh(), tea.RequestBackgroundColor)
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.applyColorScheme(msg.IsDark())
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case actionDoneMsg:
		m.loading = false
		m.loadingDir = ""
		m.loadingMsg = ""
		m.message = msg.text
		m.messageIsErr = false
		return m, tea.Batch(m.refreshStatusCmd(), m.fetchLogsCmd())
	case actionErrMsg:
		m.loading = false
		m.loadingDir = ""
		m.loadingMsg = ""
		m.message = msg.err.Error()
		m.messageIsErr = true
	case attachReadyMsg:
		m.loading = false
		m.loadingDir = ""
		m.loadingMsg = ""
		m.attachSpec = &msg.spec
		m.quitInfo = ""
		return m, tea.Quit
	case groupCreatedMsg:
		m.loading = false
		m.loadingDir = ""
		m.loadingMsg = ""
		m.createGroupMode = false
		m.createGroupSelected = map[string]bool{}
		m.createGroupInput.Blur()
		m.rc.project = msg.project
		m.rc.manager = runtime.NewManager(msg.project, m.rc.repoRoot, m.rc.worktrees, m.rc.newBackend())
		m.targets = m.rc.project.Targets()
		m.selectTarget(msg.target)
		m.message = "created group " + msg.target.Name
		m.messageIsErr = false
		return m, tea.Batch(m.refreshStatusCmd(), m.fetchLogsCmd())
	case statusRefreshedMsg:
		if msg.request != m.statusRequest {
			return m, nil
		}
		m.statusPending = false
		if msg.err != nil {
			m.message = msg.err.Error()
			m.messageIsErr = true
			return m, nil
		}
		currentDir := ""
		if cur := m.current(); cur != nil {
			currentDir = cur.Dir
		}
		if msg.worktrees != nil {
			m.rc.worktrees = append([]gitwt.Worktree(nil), msg.worktrees...)
			m.rc.manager.UpdateWorktrees(msg.worktrees)
		}
		m.rows = msg.rows
		if len(msg.rows) == 0 {
			m.idx = 0
			return m, nil
		}
		for i := range msg.rows {
			if msg.rows[i].Dir == currentDir {
				m.idx = i
				return m, nil
			}
		}
		if m.idx >= len(msg.rows) {
			m.idx = len(msg.rows) - 1
		}
	case logsMsg:
		if msg.request == m.logRequest {
			m.logPending = false
		}
		cur := m.current()
		if msg.request == m.logRequest && cur != nil && msg.dir == cur.Dir {
			m.logLines = msg.linesByTarget
			m.logDir = msg.dir
		}
	case tickLogsMsg:
		var status, logs tea.Cmd
		if !m.statusPending {
			status = m.refreshStatusCmd()
		}
		if !m.logPending {
			logs = m.fetchLogsCmd()
		}
		return m, tea.Batch(status, logs, m.scheduleLogRefresh())
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.buildQuitInfo()
			return m, tea.Quit
		}
		if m.loading {
			if !m.createGroupMode && !m.filterMode && key.Matches(msg, m.keys.Quit) {
				m.buildQuitInfo()
				return m, tea.Quit
			}
			return m, nil
		}
		if m.createGroupMode {
			return m.updateCreateGroupKeys(msg)
		}
		if m.filterMode {
			return m.updateFilterKeys(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.buildQuitInfo()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showAll = !m.showAll
		case key.Matches(msg, m.keys.Next):
			m.next()
			return m, m.fetchLogsCmd()
		case key.Matches(msg, m.keys.Prev):
			m.prev()
			return m, m.fetchLogsCmd()
		case key.Matches(msg, m.keys.Switch):
			return m, m.switchCurrentCmd()
		case key.Matches(msg, m.keys.Restart):
			return m, m.restartCurrentCmd()
		case key.Matches(msg, m.keys.StopAll):
			return m, m.stopAllCurrentCmd()
		case key.Matches(msg, m.keys.Stop):
			return m, m.stopCurrentCmd()
		case key.Matches(msg, m.keys.Attach):
			return m, m.attachCurrentCmd()
		case key.Matches(msg, m.keys.ProcNext):
			m.cycleTarget(1)
			return m, m.fetchLogsCmd()
		case key.Matches(msg, m.keys.ProcPrev):
			m.cycleTarget(-1)
			return m, m.fetchLogsCmd()
		case key.Matches(msg, m.keys.Filter):
			return m, m.enterFilterMode()
		case key.Matches(msg, m.keys.CreateGroup):
			return m, m.enterCreateGroupMode()
		}
	}

	if m.createGroupMode {
		var cmd tea.Cmd
		m.createGroupInput, cmd = m.createGroupInput.Update(msg)
		return m, cmd
	}
	if m.filterMode {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// --- Navigation ---

func (m *tuiModel) next() {
	if len(m.rows) == 0 {
		return
	}
	m.idx = (m.idx + 1) % len(m.rows)
	m.logLines = nil
}

func (m *tuiModel) prev() {
	if len(m.rows) == 0 {
		return
	}
	m.idx = (m.idx - 1 + len(m.rows)) % len(m.rows)
	m.logLines = nil
}

func (m *tuiModel) current() *runtime.StatusRow {
	if len(m.rows) == 0 || m.idx < 0 || m.idx >= len(m.rows) {
		return nil
	}
	return &m.rows[m.idx]
}

func (m *tuiModel) selectedTarget() (model.Target, bool) {
	if len(m.targets) == 0 || m.targetIdx < 0 || m.targetIdx >= len(m.targets) {
		return model.Target{}, false
	}
	return m.targets[m.targetIdx], true
}

func targetProcessState(row *runtime.StatusRow, target model.Target) (managed, exited bool) {
	if row == nil || target.Name == "" {
		return false, false
	}
	members := make(map[string]struct{}, max(1, len(target.ProcessNames)))
	for _, name := range target.ProcessNames {
		members[name] = struct{}{}
	}
	if len(members) == 0 {
		members[target.Name] = struct{}{}
	}
	allExited := true
	for _, process := range row.Processes {
		if _, matches := members[process.Name]; !matches || !process.Running {
			continue
		}
		managed = true
		if !process.Exited {
			allExited = false
		}
	}
	return managed, managed && allExited
}

func (m *tuiModel) selectTarget(target model.Target) {
	for i := range m.targets {
		if sameTarget(m.targets[i], target) {
			m.targetIdx = i
			return
		}
	}
}

func (m *tuiModel) cycleTarget(delta int) {
	if len(m.targets) == 0 {
		m.message = "no targets configured — add processes or groups to .wts.yaml"
		m.messageIsErr = true
		return
	}
	if m.targetIdx < 0 {
		m.targetIdx = 0
	} else {
		m.targetIdx = (m.targetIdx + delta + len(m.targets)) % len(m.targets)
	}
	m.logLines = nil
	m.message = ""
	m.messageIsErr = false
}

// renderGroupLogs shares the viewport fairly, redistributing unused lines.
// Process tails stay separate: capture-pane does not supply timestamps to merge.
func (m *tuiModel) renderGroupLogs(target model.Target, logSpace, maxW int) []string {
	if logSpace <= 0 || maxW <= 0 {
		return nil
	}
	counts := make([]int, len(target.ProcessNames))
	for budget := logSpace; budget > 0; {
		allocated := false
		for i, name := range target.ProcessNames {
			if budget > 0 && counts[i] < len(m.logLines[name]) {
				counts[i]++
				budget--
				allocated = true
			}
		}
		if !allocated {
			break
		}
	}
	lines := make([]string, 0, logSpace)
	for i, name := range target.ProcessNames {
		tail := m.logLines[name]
		for _, line := range tail[len(tail)-counts[i]:] {
			tag := m.styles.dimText.Render(truncateLine(name, max(1, maxW/3)) + " │ ")
			lines = append(lines, truncateLine(tag+m.styles.logText.Render(truncateLine(line, max(0, maxW-lipgloss.Width(tag)))), maxW))
		}
	}
	if len(lines) == 0 {
		return []string{m.styles.dimText.Render(truncateLine("Waiting for output…", maxW))}
	}
	return lines
}

func sameTarget(left, right model.Target) bool {
	return left.Kind == right.Kind && left.Name == right.Name
}

func formatTargetLabel(target model.Target) string {
	if target.Kind == model.TargetGroup {
		return "[group] " + target.Name
	}
	return target.Name
}

func runOptionsForTarget(target model.Target) runtime.RunOptions {
	opts := runtime.RunOptions{}
	if target.Kind == model.TargetGroup {
		opts.Group = target.Name
		return opts
	}
	opts.Process = target.Name
	return opts
}
