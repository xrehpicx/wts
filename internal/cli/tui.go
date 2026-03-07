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

	"github.com/xrehpicx/wts/internal/config"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

type actionDoneMsg struct{ text string }
type actionErrMsg struct{ err error }
type attachReadyMsg struct{ spec runtime.AttachSpec }
type groupCreatedMsg struct {
	project *model.Project
	target  model.Target
}
type statusRefreshedMsg struct {
	rows []runtime.StatusRow
	err  error
}
type logsMsg struct {
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
	rc                  *runtimeContext
	idx                 int
	width               int
	height              int
	keys                tuiKeyMap
	help                help.Model
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

func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Prev, k.Switch, k.Restart, k.Stop, k.StopAll, k.Attach, k.ProcPrev, k.ProcNext, k.Filter, k.CreateGroup, k.Help, k.Quit}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev, k.Switch, k.Restart, k.Stop, k.StopAll, k.Attach},
		{k.ProcPrev, k.ProcNext, k.Filter, k.CreateGroup, k.Help, k.Quit},
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

func newTUIModel(rc *runtimeContext) *tuiModel {
	helpModel := help.New()
	helpModel.ShowAll = false
	// Use basic ANSI colors inherited from the terminal for the shortcut line.
	helpModel.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true)
	helpModel.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpModel.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpModel.Styles.FullKey = helpModel.Styles.ShortKey
	helpModel.Styles.FullDesc = helpModel.Styles.ShortDesc
	helpModel.Styles.FullSeparator = helpModel.Styles.ShortSeparator

	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"◒", "◐", "◓", "◑"},
		FPS:    80 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FFD392"})

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64

	createGroupInput := textinput.New()
	createGroupInput.Prompt = ""
	createGroupInput.CharLimit = 64

	m := &tuiModel{
		rc:                  rc,
		keys:                newTUIKeyMap(),
		help:                helpModel,
		styles:              newTUIStyles(),
		spinner:             s,
		filterInput:         ti,
		targetIdx:           -1,
		createGroupInput:    createGroupInput,
		createGroupSelected: map[string]bool{},
	}

	targets := rc.project.Targets()
	if activeTarget, ok := rc.manager.ActiveTarget(context.Background()); ok {
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
				if activeTarget, ok := rc.manager.ActiveTarget(context.Background()); ok {
					m.selectTarget(activeTarget)
				}
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
			m.logLines = msg.linesByTarget
			m.logDir = msg.dir
		}
	case tickLogsMsg:
		return m, tea.Batch(m.refreshStatusCmd(), m.fetchLogsCmd(), m.scheduleLogRefresh())
	case tea.KeyMsg:
		if m.createGroupMode {
			return m.updateCreateGroupKeys(msg)
		}
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
		case key.Matches(msg, m.keys.StopAll):
			return m, m.stopAllCurrentCmd()
		case key.Matches(msg, m.keys.Stop):
			return m, m.stopCurrentCmd()
		case key.Matches(msg, m.keys.Attach):
			return m, m.attachCurrentCmd()
		case key.Matches(msg, m.keys.ProcNext):
			m.cycleTarget(1)
		case key.Matches(msg, m.keys.ProcPrev):
			m.cycleTarget(-1)
		case key.Matches(msg, m.keys.Filter):
			return m, m.enterFilterMode()
		case key.Matches(msg, m.keys.CreateGroup):
			return m, m.enterCreateGroupMode()
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
		target, ok := m.selectedTarget()
		active := m.activeRow()
		var summary string
		targetLabel := "no target"
		if ok {
			targetLabel = formatTargetLabel(target)
		}
		if active != nil {
			wt := m.styles.metaValue.Render(active.Worktree)
			branch := m.styles.dimText.Render(" [" + active.Branch + "]")
			var dot string
			nprocs := len(active.Processes)
			if active.Running && active.Exited {
				dot = m.styles.exitedDot.Render(" · ● exited")
			} else if active.Running && nprocs > 1 {
				dot = m.styles.runDot.Render(fmt.Sprintf(" · ● %d running", nprocs))
			} else if active.Running {
				dot = m.styles.runDot.Render(" · ● running")
			} else {
				dot = m.styles.stopDot.Render(" · ○ stopped")
			}
			summary = m.styles.title.Render(targetLabel) + m.styles.dimText.Render(" → ") + wt + branch + dot
		} else if !ok {
			summary = m.styles.dimText.Render("select a process or group with ←/→")
		} else {
			summary = m.styles.title.Render(targetLabel) + m.styles.dimText.Render(" (idle)")
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
	leftWidth := max(28, (usableWidth*3)/10)
	rightWidth := max(30, usableWidth-leftWidth)
	if leftWidth+rightWidth > width {
		rightWidth = usableWidth - leftWidth
	}

	left := m.renderListPanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	if m.createGroupMode {
		right = m.renderCreateGroupPanel(rightWidth, height)
	}
	left = lipgloss.NewStyle().MarginRight(spacer).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *tuiModel) renderListPanel(width, height int) string {
	maxTextWidth := max(12, width-4)
	lines := make([]string, 0, len(m.rows)*3)

	// Compute display names, disambiguating when names collide.
	nameCount := map[string]int{}
	for _, r := range m.rows {
		nameCount[r.Worktree]++
	}
	displayNames := make([]string, len(m.rows))
	for i, r := range m.rows {
		if nameCount[r.Worktree] > 1 {
			parent := filepath.Base(filepath.Dir(r.Dir))
			displayNames[i] = r.Worktree + " (" + parent + ")"
		} else {
			displayNames[i] = r.Worktree
		}
	}

	for i := range m.rows {
		row := m.rows[i]

		// --- Line 1: cursor + dot + name + badge ---
		cursor := "  "
		if i == m.idx {
			cursor = "▸ "
		}

		var dot string
		if row.Prunable {
			dot = m.styles.exitedDot.Render("⚠")
		} else if m.loading && row.Dir == m.loadingDir {
			dot = m.spinner.View()
		} else if row.Running && row.Exited {
			dot = m.styles.exitedDot.Render("●")
		} else if row.Running {
			dot = m.styles.runDot.Render("●")
		} else {
			dot = m.styles.stopDot.Render("○")
		}

		nameText := truncateLine(displayNames[i], max(1, maxTextWidth-6))
		namePart := cursor + dot + " " + nameText

		// Right-aligned badge.
		procBadge := ""
		if row.Prunable {
			procBadge = "prunable"
		} else if len(row.Processes) > 1 {
			procBadge = fmt.Sprintf("×%d", len(row.Processes))
		} else if row.Active {
			procBadge = "★"
		}

		var line1 string
		if procBadge != "" {
			nameW := lipgloss.Width(namePart)
			badgeW := lipgloss.Width(procBadge)
			gap := max(1, maxTextWidth-nameW-badgeW)
			line1 = namePart + strings.Repeat(" ", gap) + m.styles.dimText.Render(procBadge)
		} else {
			line1 = namePart
		}

		// --- Line 2: branch (indented, dimmed) + process names ---
		branchIndent := "     "
		availW := max(1, maxTextWidth-len(branchIndent))
		branchText := truncateLine(row.Branch, availW)

		var line2 string
		if row.Prunable {
			line2 = branchIndent + m.styles.dimText.Render(branchText)
		} else if len(row.Processes) > 0 && row.Running {
			// Show compact process status dots after branch.
			procParts := make([]string, 0, len(row.Processes))
			for _, p := range row.Processes {
				var pdot string
				if p.Running && p.Exited {
					pdot = m.styles.exitedDot.Render("●")
				} else if p.Running {
					pdot = m.styles.runDot.Render("●")
				} else {
					pdot = m.styles.stopDot.Render("○")
				}
				procParts = append(procParts, pdot+" "+m.styles.dimText.Render(p.Name))
			}
			procInfo := strings.Join(procParts, m.styles.dimText.Render(" · "))
			branchLine := m.styles.dimText.Render(branchText)
			sep := m.styles.dimText.Render(" · ")
			combined := branchLine + sep + procInfo
			if lipgloss.Width(combined) > availW {
				line2 = branchIndent + m.styles.dimText.Render(branchText)
			} else {
				line2 = branchIndent + combined
			}
		} else {
			line2 = branchIndent + m.styles.dimText.Render(branchText)
		}

		// Apply selection styling padded to full width for uniform highlight.
		if i == m.idx {
			sel := m.styles.selectedRow.Width(maxTextWidth)
			line1 = sel.Render(line1)
			line2 = sel.Render(line2)
		}

		lines = append(lines, line1, line2)

		// Add a blank separator between entries (except after the last one).
		if i < len(m.rows)-1 {
			lines = append(lines, "")
		}
	}

	return m.renderPanel("Worktrees", lines, width, height, true)
}

func (m *tuiModel) renderDetailPanel(width, height int) string {
	maxW := max(12, width-4)
	target, ok := m.selectedTarget()
	panelTitle := "←/→ to select process or group"
	if ok {
		panelTitle = formatTargetLabel(target)
	}

	row := m.current()
	if row == nil {
		return m.renderPanel(panelTitle,
			[]string{m.styles.dimText.Render("No worktree selected.")},
			width, height, false)
	}

	innerHeight := max(1, height-2)
	capacity := innerHeight - 2
	if capacity < 4 {
		return m.renderPanel(panelTitle,
			[]string{m.styles.dimText.Render(row.Worktree)},
			width, height, false)
	}

	// Meta line: branch · dir
	meta := m.styles.metaValue.Render(row.Branch) +
		m.styles.dimText.Render(" · ") +
		m.styles.dimText.Render(truncateLine(shortenPath(row.Dir), max(1, maxW-lipgloss.Width(row.Branch)-4)))

	// Running processes summary
	var procSummary string
	if len(row.Processes) > 0 {
		parts := make([]string, 0, len(row.Processes))
		for _, p := range row.Processes {
			var dot string
			if p.Running && p.Exited {
				dot = m.styles.exitedDot.Render("●")
			} else if p.Running {
				dot = m.styles.runDot.Render("●")
			} else {
				dot = m.styles.stopDot.Render("○")
			}
			parts = append(parts, dot+" "+p.Name)
		}
		procSummary = strings.Join(parts, m.styles.dimText.Render("  "))
	} else if !row.Running {
		procSummary = m.styles.stopDot.Render("○") + m.styles.dimText.Render(" no processes running")
	}

	// Command for selected process
	detailLines := make([]string, 0, 3)
	switch {
	case !ok:
		detailLines = append(detailLines, m.styles.dimText.Render("← / → to select a process or group"))
	case target.Kind == model.TargetGroup:
		members := truncateLine(strings.Join(target.ProcessNames, ", "), maxW)
		detailLines = append(detailLines, m.styles.dimText.Render("members: ")+m.styles.metaValue.Render(members))
	default:
		procDef, err := m.rc.project.Process(target.Name)
		if err != nil {
			detailLines = append(detailLines, m.styles.statusErr.Render(truncateLine(err.Error(), maxW)))
		} else {
			detailLines = append(detailLines, m.styles.dimText.Render("▸ ")+m.styles.metaValue.Render(truncateLine(procDef.Command, maxW-2)))
		}
	}

	// Output separator
	label := " output "
	if ok && target.Kind == model.TargetProcess {
		label = " " + target.Name + " "
	} else if ok {
		label = " " + target.Name + " "
	}
	sepW := max(0, maxW-runeLen(label))
	leftSep := max(0, sepW/5)
	rightSep := max(0, sepW-leftSep)
	outputSep := m.styles.separator.Render(strings.Repeat("─", leftSep)) +
		m.styles.dimText.Render(label) +
		m.styles.separator.Render(strings.Repeat("─", rightSep))

	// Action hint
	var hint string
	targetNoun := "target"
	if ok && target.Kind == model.TargetGroup {
		targetNoun = "group"
	} else if ok {
		targetNoun = "process"
	}
	if row.Running && row.Exited {
		hint = m.styles.dimText.Render("a attach tmux · r restart · x stop · " + targetNoun + " exited")
	} else if row.Running {
		hint = m.styles.dimText.Render("s/↵ add " + targetNoun + " · a attach tmux · r restart · x stop")
	} else {
		hint = m.styles.dimText.Render("s/↵ start " + targetNoun)
	}

	// Build lines: meta(1) + procs(1) + cmd(1) + sep(1) + [logs...] + hint(1)
	lines := make([]string, 0, capacity)
	lines = append(lines, meta)
	if procSummary != "" {
		lines = append(lines, procSummary)
	}
	lines = append(lines, detailLines...)
	lines = append(lines, outputSep)

	logSpace := capacity - len(lines) - 1
	if logSpace > 0 {
		cur := m.current()
		if cur != nil && cur.Dir == m.logDir && len(m.logLines) > 0 {
			if ok && target.Kind == model.TargetGroup {
				lines = append(lines, m.renderGroupLogs(target, logSpace, maxW)...)
			} else {
				processName := ""
				if ok {
					processName = target.Name
				}
				processLogs := m.logLines[processName]
				start := max(0, len(processLogs)-logSpace)
				for _, l := range processLogs[start:] {
					lines = append(lines, m.styles.logText.Render(truncateLine(l, maxW)))
				}
			}
		} else if !row.Running {
			lines = append(lines, m.styles.dimText.Render(targetNoun+" not running"))
		}
	}

	for len(lines) < capacity-1 {
		lines = append(lines, "")
	}
	lines = append(lines, hint)

	return m.renderPanel(panelTitle, lines, width, height, false)
}

func (m *tuiModel) renderCreateGroupPanel(width, height int) string {
	maxW := max(12, width-4)
	innerHeight := max(1, height-2)
	capacity := innerHeight - 2

	lines := []string{
		m.styles.dimText.Render("Create a group in " + filepath.Base(m.rc.project.ConfigPath)),
		"",
		m.styles.dimText.Render("name"),
		m.renderCreateGroupNameLine(maxW),
		"",
		m.styles.dimText.Render("members"),
	}

	processNames := m.rc.project.ProcessNames()
	if len(processNames) == 0 {
		lines = append(lines, m.styles.statusErr.Render("No processes available"))
	} else {
		for i, name := range processNames {
			cursor := "  "
			if m.createGroupFocus == createGroupFocusMembers && i == m.createGroupCursor {
				cursor = "▸ "
			}
			box := "[ ]"
			if m.createGroupSelected[name] {
				box = "[x]"
			}
			line := cursor + box + " " + name
			if m.createGroupFocus == createGroupFocusMembers && i == m.createGroupCursor {
				line = m.styles.modalFocus.Render(truncateLine(line, maxW))
			} else {
				line = m.styles.row.Render(truncateLine(line, maxW))
			}
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	lines = append(lines, m.styles.dimText.Render("tab switch focus · space toggle member · enter save · esc cancel"))

	if len(lines) > capacity {
		lines = lines[:capacity]
	}
	for len(lines) < capacity {
		lines = append(lines, "")
	}

	return m.styles.modalBorder.Width(width).Render(strings.Join(append([]string{m.styles.panelTitle.Render("Create Group"), ""}, lines...), "\n"))
}

func (m *tuiModel) renderCreateGroupNameLine(maxW int) string {
	line := m.createGroupInput.View()
	if strings.TrimSpace(line) == "" {
		line = m.styles.dimText.Render("group name")
	}
	line = truncateLine(line, maxW)
	if m.createGroupFocus == createGroupFocusName {
		return m.styles.modalFocus.Render(line)
	}
	return m.styles.row.Render(line)
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
	return " " + helpView
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

func (m *tuiModel) selectedTarget() (model.Target, bool) {
	if len(m.targets) == 0 || m.targetIdx < 0 || m.targetIdx >= len(m.targets) {
		return model.Target{}, false
	}
	return m.targets[m.targetIdx], true
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
	target, _ := m.selectedTarget()
	m.message = "target: " + formatTargetLabel(target)
	m.messageIsErr = false
}

func (m *tuiModel) renderGroupLogs(target model.Target, logSpace, maxW int) []string {
	if len(target.ProcessNames) == 0 || logSpace <= 0 {
		return nil
	}

	// Build a tag style per process using the process name as a prefix.
	// Interleave the most recent lines from each process to show a unified
	// chronological-ish view (latest output at the bottom).

	// Collect tail lines from each process, most-recent-last.
	type taggedLine struct {
		tag  string
		text string
	}
	var merged []taggedLine
	nprocs := len(target.ProcessNames)

	// Give each process a fair share of lines, but let any process use
	// surplus space if another has fewer lines.
	budget := logSpace
	remaining := make([]string, 0, nprocs)
	for _, name := range target.ProcessNames {
		if len(m.logLines[name]) > 0 {
			remaining = append(remaining, name)
		}
	}
	if len(remaining) == 0 {
		return []string{m.styles.dimText.Render("waiting for output...")}
	}

	perProc := max(1, budget/len(remaining))
	for _, name := range remaining {
		plog := m.logLines[name]
		n := min(perProc, len(plog))
		start := len(plog) - n
		for _, l := range plog[start:] {
			merged = append(merged, taggedLine{tag: name, text: l})
		}
	}

	// Trim to fit.
	if len(merged) > logSpace {
		merged = merged[len(merged)-logSpace:]
	}

	// Compute the shortest unambiguous tag for each process name.
	shortTag := make(map[string]string, nprocs)
	for _, name := range target.ProcessNames {
		shortTag[name] = name
	}

	lines := make([]string, 0, len(merged))
	for _, ml := range merged {
		tag := m.styles.dimText.Render(shortTag[ml.tag] + " │ ")
		tagW := lipgloss.Width(tag)
		textW := max(1, maxW-tagW)
		lines = append(lines, tag+m.styles.logText.Render(truncateLine(ml.text, textW)))
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

func (m *tuiModel) updateCreateGroupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
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
		switch msg.Type {
		case tea.KeyUp:
			if len(m.rc.project.Processes) > 0 {
				m.createGroupCursor = (m.createGroupCursor - 1 + len(m.rc.project.Processes)) % len(m.rc.project.Processes)
			}
			return m, nil
		case tea.KeyDown:
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

	return func() tea.Msg {
		project, err := config.Save(m.rc.project.ConfigPath, cfg)
		if err != nil {
			return actionErrMsg{err: err}
		}
		target, err := project.ResolveTarget("", name)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return groupCreatedMsg{project: project, target: target}
	}
}

// --- Process filter ---

func (m *tuiModel) enterFilterMode() tea.Cmd {
	m.filterMode = true
	m.preFilterIdx = m.targetIdx
	m.filterInput.SetValue("")
	return m.filterInput.Focus()
}

func (m *tuiModel) updateFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.filterMode = false
		m.filterInput.Blur()
		target, ok := m.selectedTarget()
		if ok {
			m.message = "target: " + formatTargetLabel(target)
		} else {
			m.message = ""
		}
		m.messageIsErr = false
		return m, nil
	case tea.KeyEscape:
		m.filterMode = false
		m.filterInput.Blur()
		m.targetIdx = m.preFilterIdx
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
	for i, target := range m.targets {
		if strings.Contains(strings.ToLower(formatTargetLabel(target)), q) {
			m.targetIdx = i
			return
		}
	}
}

func (m *tuiModel) countFilterMatches(query string) int {
	if query == "" {
		return len(m.targets)
	}
	q := strings.ToLower(query)
	count := 0
	for _, target := range m.targets {
		if strings.Contains(strings.ToLower(formatTargetLabel(target)), q) {
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

	action := func() tea.Msg {
		opts := runOptionsForTarget(target)
		var err error
		if useAdditive {
			err = m.rc.manager.Start(context.Background(), dir, opts)
		} else {
			err = m.rc.manager.Switch(context.Background(), dir, opts)
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
	action := func() tea.Msg {
		if err := m.rc.manager.Restart(context.Background(), dir, runOptionsForTarget(target)); err != nil {
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
	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "stopping " + formatTargetLabel(target) + " in " + name + "..."
	action := func() tea.Msg {
		var err error
		if target.Kind == model.TargetGroup {
			err = m.rc.manager.StopGroup(context.Background(), dir, target.Name)
		} else {
			err = m.rc.manager.StopProcess(context.Background(), dir, target.Name)
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
	action := func() tea.Msg {
		if err := m.rc.manager.StopWorktree(context.Background(), dir); err != nil {
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
	if !row.Running {
		m.message = "selected worktree is not running"
		m.messageIsErr = true
		return nil
	}

	dir, name := row.Dir, row.Worktree
	m.loading = true
	m.loadingDir = dir
	m.loadingMsg = "attaching " + formatTargetLabel(target) + " in " + name + "..."

	action := func() tea.Msg {
		spec, err := m.rc.manager.ResolveAttach(context.Background(), dir, runOptionsForTarget(target))
		if err != nil {
			return actionErrMsg{err: err}
		}
		return attachReadyMsg{spec: spec}
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
	target, ok := m.selectedTarget()
	return func() tea.Msg {
		linesByTarget := map[string][]string{}
		if ok && target.Kind == model.TargetGroup {
			for _, processName := range target.ProcessNames {
				output, err := m.rc.manager.Logs(context.Background(), dir, processName, 200)
				if err != nil {
					continue
				}
				raw := strings.TrimRight(output, "\n")
				if raw == "" {
					continue
				}
				linesByTarget[processName] = strings.Split(raw, "\n")
			}
			return logsMsg{dir: dir, linesByTarget: linesByTarget}
		}

		processName := ""
		if ok {
			processName = target.Name
		}
		output, err := m.rc.manager.Logs(context.Background(), dir, processName, 200)
		if err != nil {
			return logsMsg{dir: dir, linesByTarget: linesByTarget}
		}
		raw := strings.TrimRight(output, "\n")
		if raw == "" {
			return logsMsg{dir: dir, linesByTarget: linesByTarget}
		}
		linesByTarget[processName] = strings.Split(raw, "\n")
		return logsMsg{dir: dir, linesByTarget: linesByTarget}
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
