package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

type actionDoneMsg struct{ text string }
type actionErrMsg struct{ err error }
type statusRefreshedMsg struct {
	rows []runtime.StatusRow
	err  error
}
type logsMsg struct {
	dir   string
	lines []string
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
}

type tuiModel struct {
	rc           *runtimeContext
	idx          int
	width        int
	height       int
	keys         tuiKeyMap
	help         help.Model
	showAll      bool
	message      string
	messageIsErr bool
	rows         []runtime.StatusRow
	processNames []string
	processIdx   int
	styles       tuiStyles
	spinner      spinner.Model
	loading      bool
	loadingDir   string
	loadingMsg   string
	quitInfo     string
	filterMode   bool
	filterInput  textinput.Model
	preFilterIdx int
	logLines     []string
	logDir       string
}

type tuiKeyMap struct {
	Next     key.Binding
	Prev     key.Binding
	Switch   key.Binding
	Restart  key.Binding
	Stop     key.Binding
	ProcPrev key.Binding
	ProcNext key.Binding
	Filter   key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func newTUIKeyMap() tuiKeyMap {
	return tuiKeyMap{
		Next:     key.NewBinding(key.WithKeys("n", "down"), key.WithHelp("n/↓", "next")),
		Prev:     key.NewBinding(key.WithKeys("p", "up"), key.WithHelp("p/↑", "prev")),
		Switch:   key.NewBinding(key.WithKeys("s", "enter"), key.WithHelp("s/↵", "switch")),
		Restart:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
		Stop:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		ProcPrev: key.NewBinding(key.WithKeys("left", "["), key.WithHelp("←/[", "prev process")),
		ProcNext: key.NewBinding(key.WithKeys("right", "]"), key.WithHelp("→/]", "next process")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search process")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Prev, k.Switch, k.Restart, k.Stop, k.ProcPrev, k.ProcNext, k.Filter, k.Help, k.Quit}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev, k.Switch, k.Restart, k.Stop},
		{k.ProcPrev, k.ProcNext, k.Filter, k.Help, k.Quit},
	}
}

func newTUIStyles() tuiStyles {
	ac := func(light, dark string) lipgloss.AdaptiveColor {
		return lipgloss.AdaptiveColor{Light: light, Dark: dark}
	}
	return tuiStyles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ac("#0B3954", "#D9ECFF")),
		subtitle: lipgloss.NewStyle().
			Foreground(ac("#4A5568", "#6B7F96")),
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
		colHeader: lipgloss.NewStyle().Foreground(ac("#3E5C76", "#6B8AAE")).Bold(true),
		runDot:    lipgloss.NewStyle().Foreground(ac("#166534", "#8FE3B2")),
		exitedDot: lipgloss.NewStyle().Foreground(ac("#92400E", "#FFD392")),
		stopDot:   lipgloss.NewStyle().Foreground(ac("#6B7280", "#4B5C6E")),
		activeMark: lipgloss.NewStyle().
			Foreground(ac("#92400E", "#FFD392")).
			Bold(true),
		metaLabel: lipgloss.NewStyle().
			Foreground(ac("#46607A", "#6B8AAE")).
			Width(10),
		metaValue: lipgloss.NewStyle().
			Foreground(ac("#0F172A", "#E6EEF7")),
		separator: lipgloss.NewStyle().
			Foreground(ac("#D1D5DB", "#2D3F54")),
		dimText: lipgloss.NewStyle().
			Foreground(ac("#6B7280", "#4B5C6E")),
		logText: lipgloss.NewStyle().
			Foreground(ac("#374151", "#9CAABB")),
		footer: lipgloss.NewStyle().
			Foreground(ac("#4B5563", "#6B7F96")),
	}
}

func newTUIModel(rc *runtimeContext) *tuiModel {
	helpModel := help.New()
	helpModel.ShowAll = false

	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"◒", "◐", "◓", "◑"},
		FPS:    80 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FFD392"})

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64

	m := &tuiModel{
		rc:          rc,
		keys:        newTUIKeyMap(),
		help:        helpModel,
		styles:      newTUIStyles(),
		spinner:     s,
		filterInput: ti,
		processIdx:  -1,
	}

	// Order processes: active process first, then config order.
	names := rc.project.ProcessNames()
	activeProc := rc.manager.ActiveProcess(context.Background())
	if activeProc != "" {
		reordered := make([]string, 0, len(names))
		reordered = append(reordered, activeProc)
		for _, n := range names {
			if n != activeProc {
				reordered = append(reordered, n)
			}
		}
		m.processNames = reordered
	} else {
		m.processNames = names
	}

	m.refreshStatus()
	if len(m.rows) > 0 {
		for i := range m.rows {
			if m.rows[i].Active {
				m.idx = i
				m.selectProcessByName(m.rows[i].Process)
				break
			}
		}
	}
	return m
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(m.fetchLogsCmd(), m.scheduleLogRefresh())
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	case statusRefreshedMsg:
		if msg.err != nil {
			m.message = msg.err.Error()
			m.messageIsErr = true
			return m, nil
		}
		currentDir := ""
		if cur := m.current(); cur != nil {
			currentDir = cur.Dir
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
		cur := m.current()
		if cur != nil && msg.dir == cur.Dir {
			m.logLines = msg.lines
			m.logDir = msg.dir
		}
	case tickLogsMsg:
		return m, tea.Batch(m.fetchLogsCmd(), m.scheduleLogRefresh())
	case tea.KeyMsg:
		if m.filterMode {
			return m.updateFilterKeys(msg)
		}
		if m.loading {
			if key.Matches(msg, m.keys.Quit) {
				m.buildQuitInfo()
				return m, tea.Quit
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.buildQuitInfo()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showAll = !m.showAll
			m.help.ShowAll = m.showAll
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
		case key.Matches(msg, m.keys.Stop):
			return m, m.stopCurrentCmd()
		case key.Matches(msg, m.keys.ProcNext):
			m.cycleProcess(1)
		case key.Matches(msg, m.keys.ProcPrev):
			m.cycleProcess(-1)
		case key.Matches(msg, m.keys.Filter):
			return m, m.enterFilterMode()
		}
	}

	if m.filterMode {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *tuiModel) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 110
	}
	if h <= 0 {
		h = 30
	}

	header := m.renderHeader(w)
	footer := m.renderFooter(w)

	contentH := max(6, h-lipgloss.Height(header)-lipgloss.Height(footer))
	content := m.renderContent(w, contentH)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// --- Render sections ---

func (m *tuiModel) renderHeader(width int) string {
	clamp := lipgloss.NewStyle().MaxWidth(width)
	repoName := filepath.Base(m.rc.repoRoot)

	topLeft := " " + m.styles.title.Render("wts") + m.styles.dimText.Render(" · "+repoName)
	topRight := m.styles.subtitle.Render(fmt.Sprintf("%d worktrees", len(m.rows))) + " "

	row1 := clamp.Render(headerRow(topLeft, topRight, width))

	var botLeft, botRight string

	if m.filterMode {
		botLeft = " " + m.styles.dimText.Render("/") + " " + m.filterInput.View()
		count := m.countFilterMatches(m.filterInput.Value())
		botRight = m.styles.subtitle.Render(fmt.Sprintf("%d matching", count)) + " "
	} else {
		proc := m.selectedProcess()
		active := m.activeRow()
		var summary string
		procLabel := proc
		if procLabel == "" {
			procLabel = "no process"
		}
		if active != nil {
			wt := m.styles.metaValue.Render(active.Worktree)
			branch := m.styles.dimText.Render(" [" + active.Branch + "]")
			var dot string
			if active.Running && active.Exited {
				dot = m.styles.exitedDot.Render(" · ● exited")
			} else if active.Running {
				dot = m.styles.runDot.Render(" · ● running")
			} else {
				dot = m.styles.stopDot.Render(" · ○ stopped")
			}
			summary = m.styles.title.Render(procLabel) + m.styles.dimText.Render(" → ") + wt + branch + dot
		} else if proc == "" {
			summary = m.styles.dimText.Render("select a process with ←/→")
		} else {
			summary = m.styles.title.Render(procLabel) + m.styles.dimText.Render(" (idle)")
		}
		botLeft = " " + summary

		if m.loading {
			botRight = m.styles.statusBusy.Render(m.spinner.View()+" "+m.loadingMsg) + "  "
		} else if m.message != "" {
			if m.messageIsErr {
				botRight = m.styles.statusErr.Render("✗ "+m.message) + " "
			} else {
				botRight = m.styles.statusOk.Render("✓ "+m.message) + " "
			}
		}
	}

	row2 := clamp.Render(headerRow(botLeft, botRight, width))
	sep := m.styles.separator.Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, sep)
}

func (m *tuiModel) renderContent(width, height int) string {
	if len(m.rows) == 0 {
		empty := m.styles.dimText.Render("No worktrees found. Create one with:")
		hint := m.styles.metaValue.Render("  git worktree add ../branch-name")
		return m.renderPanel("Worktrees", []string{empty, hint}, width, height, true)
	}

	spacer := 1
	usableWidth := max(20, width-spacer)
	leftWidth := max(24, (usableWidth*1)/4)
	rightWidth := max(30, usableWidth-leftWidth)
	if leftWidth+rightWidth > width {
		rightWidth = usableWidth - leftWidth
	}

	left := m.renderListPanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	left = lipgloss.NewStyle().MarginRight(spacer).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *tuiModel) renderListPanel(width, height int) string {
	maxTextWidth := max(12, width-4)
	nameWidth := min(14, maxTextWidth-10)
	lines := make([]string, 0, len(m.rows)+2)

	colHdr := fmt.Sprintf("    %-*s %s", nameWidth, "WORKTREE", "BRANCH")
	lines = append(lines, m.styles.colHeader.Render(truncateLine(colHdr, maxTextWidth)))

	for i := range m.rows {
		row := m.rows[i]
		cursor := "  "
		if i == m.idx {
			cursor = "▸ "
		}

		var dot string
		if m.loading && row.Dir == m.loadingDir {
			dot = m.spinner.View()
		} else if row.Running && row.Exited {
			dot = m.styles.exitedDot.Render("●")
		} else if row.Running {
			dot = m.styles.runDot.Render("●")
		} else {
			dot = m.styles.stopDot.Render("○")
		}

		line := fmt.Sprintf("%s%s %-*s %s",
			cursor, dot,
			nameWidth, truncateLine(row.Worktree, nameWidth),
			truncateLine(row.Branch, max(1, maxTextWidth-nameWidth-5)),
		)

		if i == m.idx {
			line = m.styles.selectedRow.Render(line)
		} else {
			line = m.styles.row.Render(line)
		}
		lines = append(lines, line)
	}

	return m.renderPanel("Worktrees", lines, width, height, true)
}

func (m *tuiModel) renderDetailPanel(width, height int) string {
	maxW := max(12, width-4)
	proc := m.selectedProcess()
	panelTitle := proc
	if panelTitle == "" {
		panelTitle = "←/→ to select process"
	}

	row := m.current()
	if row == nil {
		return m.renderPanel(panelTitle,
			[]string{m.styles.dimText.Render("No worktree selected.")},
			width, height, false)
	}

	// capacity = lines available inside the panel after title + blank
	innerHeight := max(1, height-2)
	capacity := innerHeight - 2
	if capacity < 4 {
		return m.renderPanel(panelTitle,
			[]string{m.styles.dimText.Render(row.Worktree)},
			width, height, false)
	}

	// Compact meta: [branch] · status · dir  (1 line)
	var statusDot string
	if row.Running && row.Exited {
		statusDot = m.styles.exitedDot.Render("● exited")
	} else if row.Running {
		statusDot = m.styles.runDot.Render("● running")
	} else {
		statusDot = m.styles.stopDot.Render("○ stopped")
	}
	meta := m.styles.metaValue.Render(row.Branch) +
		m.styles.dimText.Render(" · ") + statusDot +
		m.styles.dimText.Render(" · ") +
		m.styles.dimText.Render(truncateLine(shortenPath(row.Dir), max(1, maxW-lipgloss.Width(row.Branch)-22)))

	// Command (1 line, truncated)
	var cmdLine string
	if proc == "" {
		cmdLine = m.styles.dimText.Render("← / → to select a process")
	} else if procDef, err := m.rc.project.Process(proc); err != nil {
		cmdLine = m.styles.statusErr.Render(truncateLine(err.Error(), maxW))
	} else {
		cmdLine = m.styles.dimText.Render("▸ ") + m.styles.metaValue.Render(truncateLine(procDef.Command, maxW-2))
	}

	// Output separator with label
	label := " output "
	sepW := max(0, maxW-runeLen(label))
	leftSep := max(0, sepW/5)
	rightSep := max(0, sepW-leftSep)
	outputSep := m.styles.separator.Render(strings.Repeat("─", leftSep)) +
		m.styles.dimText.Render(label) +
		m.styles.separator.Render(strings.Repeat("─", rightSep))

	// Action hint (always at bottom)
	var hint string
	if row.Running && row.Exited {
		hint = m.styles.dimText.Render("r restart · x stop · process exited, shell still open")
	} else if row.Running {
		hint = m.styles.dimText.Render("r restart · x stop")
	} else {
		hint = m.styles.dimText.Render("s/↵ switch here")
	}

	// Build lines: meta(1) + cmd(1) + sep(1) + [logs...] + hint(1) = 4 fixed
	lines := make([]string, 0, capacity)
	lines = append(lines, meta, cmdLine, outputSep)

	logSpace := capacity - len(lines) - 1 // -1 for hint at bottom
	if logSpace > 0 {
		cur := m.current()
		if cur != nil && cur.Dir == m.logDir && len(m.logLines) > 0 {
			start := max(0, len(m.logLines)-logSpace)
			for _, l := range m.logLines[start:] {
				lines = append(lines, m.styles.logText.Render(truncateLine(l, maxW)))
			}
		} else if !row.Running {
			lines = append(lines, m.styles.dimText.Render("process not running"))
		}
	}

	// Pad to push hint to bottom
	for len(lines) < capacity-1 {
		lines = append(lines, "")
	}
	lines = append(lines, hint)

	return m.renderPanel(panelTitle, lines, width, height, false)
}

func (m *tuiModel) renderPanel(title string, lines []string, width, height int, focused bool) string {
	innerHeight := max(1, height-2)

	content := make([]string, 0, innerHeight)
	content = append(content, m.styles.panelTitle.Render(title))
	content = append(content, "")
	for _, line := range lines {
		content = append(content, line)
		if len(content) >= innerHeight {
			break
		}
	}
	for len(content) < innerHeight {
		content = append(content, "")
	}

	border := m.styles.panelBorder
	if focused {
		border = m.styles.panelFocus
	}
	return border.Width(width).Render(strings.Join(content, "\n"))
}

func (m *tuiModel) renderFooter(width int) string {
	m.help.Width = max(20, width)
	helpView := m.help.ShortHelpView(m.keys.ShortHelp())
	if m.showAll {
		helpView = m.help.FullHelpView(m.keys.FullHelp())
	}
	return " " + m.styles.footer.Render(helpView)
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

func (m *tuiModel) activeRow() *runtime.StatusRow {
	for i := range m.rows {
		if m.rows[i].Active {
			return &m.rows[i]
		}
	}
	return nil
}

func (m *tuiModel) selectedProcess() string {
	if len(m.processNames) == 0 || m.processIdx < 0 || m.processIdx >= len(m.processNames) {
		return ""
	}
	return m.processNames[m.processIdx]
}

func (m *tuiModel) selectProcessByName(name string) {
	for i := range m.processNames {
		if m.processNames[i] == name {
			m.processIdx = i
			return
		}
	}
}

func (m *tuiModel) cycleProcess(delta int) {
	if len(m.processNames) == 0 {
		m.message = "no processes configured — add processes to .wts.yaml"
		m.messageIsErr = true
		return
	}
	if m.processIdx < 0 {
		// First selection: go to first process regardless of delta direction.
		m.processIdx = 0
	} else {
		m.processIdx = (m.processIdx + delta + len(m.processNames)) % len(m.processNames)
	}
	m.message = "process: " + m.selectedProcess()
	m.messageIsErr = false
}

// --- Process filter ---

func (m *tuiModel) enterFilterMode() tea.Cmd {
	m.filterMode = true
	m.preFilterIdx = m.processIdx
	m.filterInput.SetValue("")
	return m.filterInput.Focus()
}

func (m *tuiModel) updateFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.filterMode = false
		m.filterInput.Blur()
		m.message = "process: " + m.selectedProcess()
		m.messageIsErr = false
		return m, nil
	case tea.KeyEscape:
		m.filterMode = false
		m.filterInput.Blur()
		m.processIdx = m.preFilterIdx
		return m, nil
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterProcesses(m.filterInput.Value())
	return m, cmd
}

func (m *tuiModel) filterProcesses(query string) {
	if query == "" {
		return
	}
	q := strings.ToLower(query)
	for i, name := range m.processNames {
		if strings.Contains(strings.ToLower(name), q) {
			m.processIdx = i
			return
		}
	}
}

func (m *tuiModel) countFilterMatches(query string) int {
	if query == "" {
		return len(m.processNames)
	}
	q := strings.ToLower(query)
	count := 0
	for _, name := range m.processNames {
		if strings.Contains(strings.ToLower(name), q) {
			count++
		}
	}
	return count
}

// --- Async actions ---

func (m *tuiModel) switchCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	proc := m.selectedProcess()
	if proc == "" {
		m.message = "select a process first with ←/→"
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "switching to " + name + "..."
	action := func() tea.Msg {
		if err := m.rc.manager.Switch(context.Background(), dir, runtime.RunOptions{Process: proc}); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "switched to " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) restartCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	proc := m.selectedProcess()
	if proc == "" {
		m.message = "select a process first with ←/→"
		m.messageIsErr = true
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "restarting " + name + "..."
	action := func() tea.Msg {
		if err := m.rc.manager.Restart(context.Background(), dir, runtime.RunOptions{Process: proc}); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "restarted " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

func (m *tuiModel) stopCurrentCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "stopping " + name + "..."
	action := func() tea.Msg {
		if err := m.rc.manager.StopWorktree(context.Background(), dir); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{text: "stopped " + name}
	}
	return tea.Batch(m.spinner.Tick, action)
}

// --- Log streaming ---

func (m *tuiModel) fetchLogsCmd() tea.Cmd {
	row := m.current()
	if row == nil {
		return nil
	}
	dir := row.Dir
	return func() tea.Msg {
		output, err := m.rc.manager.Logs(context.Background(), dir, 200)
		if err != nil {
			return logsMsg{dir: dir}
		}
		raw := strings.TrimRight(output, "\n")
		if raw == "" {
			return logsMsg{dir: dir}
		}
		return logsMsg{dir: dir, lines: strings.Split(raw, "\n")}
	}
}

func (m *tuiModel) scheduleLogRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tickLogsMsg{}
	})
}

// --- Status refresh ---

func (m *tuiModel) refreshStatusCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.rc.manager.Status(context.Background(), "")
		return statusRefreshedMsg{rows: rows, err: err}
	}
}

func (m *tuiModel) refreshStatus() {
	currentDir := ""
	if cur := m.current(); cur != nil {
		currentDir = cur.Dir
	}

	rows, err := m.rc.manager.Status(context.Background(), "")
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
		b.WriteString(fmt.Sprintf("    %s %s [%s]\n", mark, row.Worktree, row.Branch))
		b.WriteString(fmt.Sprintf("      tmux attach -t %s \\; select-window -t %s:%s\n\n", session, session, window))
	}
	m.quitInfo = b.String()
}

// --- Helpers ---

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runeLen(s) <= width {
		return s
	}
	r := []rune(s)
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func runeLen(s string) int {
	return len([]rune(s))
}

func headerRow(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap >= 1 {
		return left + strings.Repeat(" ", gap) + right
	}
	return left
}
