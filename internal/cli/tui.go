package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/state"
)

type tuiStyles struct {
	header       lipgloss.Style
	headerSub    lipgloss.Style
	panelTitle   lipgloss.Style
	panelBorder  lipgloss.Style
	panelBorderF lipgloss.Style
	selectedRow  lipgloss.Style
	row          lipgloss.Style
	colHeader    lipgloss.Style
	badgeRunning lipgloss.Style
	badgeStopped lipgloss.Style
	badgeActive  lipgloss.Style
	iconRunning  lipgloss.Style
	iconStopped  lipgloss.Style
	iconActive   lipgloss.Style
	iconInactive lipgloss.Style
	metaLabel    lipgloss.Style
	metaValue    lipgloss.Style
	footer       lipgloss.Style
	error        lipgloss.Style
	notice       lipgloss.Style
	key          lipgloss.Style
}

type tuiModel struct {
	rc      *runtimeContext
	idx     int
	width   int
	height  int
	message string
	rows    []runtime.StatusRow
	styles  tuiStyles
}

func newTUIModel(rc *runtimeContext) *tuiModel {
	idx := rc.repo.Selected
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rc.repo.Worktrees) {
		idx = max(0, len(rc.repo.Worktrees)-1)
	}
	m := &tuiModel{rc: rc, idx: idx, styles: newTUIStyles()}
	m.refreshStatus()
	return m
}

func newTUIStyles() tuiStyles {
	if os.Getenv("NO_COLOR") != "" {
		base := lipgloss.NewStyle()
		bold := lipgloss.NewStyle().Bold(true)
		return tuiStyles{
			header:       bold,
			headerSub:    base,
			panelTitle:   bold,
			panelBorder:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
			panelBorderF: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Bold(true),
			selectedRow:  bold,
			row:          base,
			colHeader:    bold,
			badgeRunning: bold,
			badgeStopped: base,
			badgeActive:  bold,
			iconRunning:  bold,
			iconStopped:  base,
			iconActive:   bold,
			iconInactive: base,
			metaLabel:    bold,
			metaValue:    base,
			footer:       base,
			error:        bold,
			notice:       base,
			key:          bold,
		}
	}

	ac := func(light, dark string) lipgloss.AdaptiveColor {
		return lipgloss.AdaptiveColor{Light: light, Dark: dark}
	}

	return tuiStyles{
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ac("#0B3954", "#B7D5FF")).
			Padding(0, 1),
		headerSub: lipgloss.NewStyle().Foreground(ac("#4A5568", "#8FA3BF")),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ac("#1D4E89", "#A4C2F4")),
		panelBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ac("#B8C5D6", "#3A4E68")).
			Padding(0, 1),
		panelBorderF: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ac("#5A84B5", "#6FA8DC")).
			Padding(0, 1),
		selectedRow: lipgloss.NewStyle().
			Foreground(ac("#102A43", "#F0F6FF")).
			Background(ac("#DCEEFF", "#24364A")).
			Bold(true),
		row:          lipgloss.NewStyle().Foreground(ac("#1F2937", "#CED7E2")),
		colHeader:    lipgloss.NewStyle().Foreground(ac("#3E5C76", "#8FB3D9")).Bold(true),
		badgeRunning: lipgloss.NewStyle().Foreground(ac("#0F5132", "#8DE1B0")).Bold(true),
		badgeStopped: lipgloss.NewStyle().Foreground(ac("#4B5563", "#9AA6B2")),
		badgeActive:  lipgloss.NewStyle().Foreground(ac("#7A4A00", "#FFD580")).Bold(true),
		iconRunning:  lipgloss.NewStyle().Foreground(ac("#0E9F6E", "#7FE8B5")).Bold(true),
		iconStopped:  lipgloss.NewStyle().Foreground(ac("#94A3B8", "#6B7C8F")),
		iconActive:   lipgloss.NewStyle().Foreground(ac("#D97706", "#FFD27D")).Bold(true),
		iconInactive: lipgloss.NewStyle().Foreground(ac("#A0AEC0", "#5E7186")),
		metaLabel:    lipgloss.NewStyle().Foreground(ac("#46607A", "#8FB0D1")),
		metaValue:    lipgloss.NewStyle().Foreground(ac("#0F172A", "#E6EEF7")),
		footer:       lipgloss.NewStyle().Foreground(ac("#4B5563", "#9AA6B2")),
		error:        lipgloss.NewStyle().Foreground(ac("#B91C1C", "#FF8B8B")).Bold(true),
		notice:       lipgloss.NewStyle().Foreground(ac("#166534", "#8DE1B0")),
		key: lipgloss.NewStyle().
			Foreground(ac("#1E3A8A", "#C7DDFF")).
			Underline(true).
			Bold(true),
	}
}

func (m *tuiModel) Init() tea.Cmd { return nil }

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
	width := m.width
	height := m.height
	if width <= 0 {
		width = 110
	}
	if height <= 0 {
		height = 30
	}

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)

	// Golden rule #1: account for panel borders in content area.
	contentHeight := max(6, height-headerHeight-footerHeight)
	content := m.renderContent(width, contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m *tuiModel) renderHeader(width int) string {
	title := m.styles.header.Render("workswitch TUI  ·  wts = worktree switch")
	runningCount := 0
	activeCount := 0
	for _, row := range m.rows {
		if row.Running {
			runningCount++
		}
		if row.Active {
			activeCount++
		}
	}
	sub := m.styles.headerSub.Render(
		truncateLine(
			fmt.Sprintf(
				"repo: %s   state: %s   running: %d/%d   active: %d",
				m.rc.repo.Root,
				m.rc.store.Path,
				runningCount,
				len(m.rc.repo.Worktrees),
				activeCount,
			),
			width,
		),
	)
	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, truncateLine(title, width), sub),
	)
}

func (m *tuiModel) renderContent(width, height int) string {
	if len(m.rc.repo.Worktrees) == 0 {
		empty := m.styles.notice.Render("No worktrees configured. Press 'a' to add repo root as a worktree entry.")
		panel := m.renderPanel("Worktrees", []string{empty}, width, height, true)
		return panel
	}

	leftWeight, rightWeight := 2, 3
	total := leftWeight + rightWeight
	spacer := 1
	usableWidth := max(20, width-spacer)
	leftWidth := max(30, (usableWidth*leftWeight)/total)
	rightWidth := max(40, usableWidth-leftWidth)
	if leftWidth+rightWidth > width {
		rightWidth = usableWidth - leftWidth
	}

	left := m.renderListPanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	return left + " " + right
}

func (m *tuiModel) renderListPanel(width, height int) string {
	lines := make([]string, 0, len(m.rc.repo.Worktrees)+2)
	maxTextWidth := max(8, width-4)
	lines = append(lines, m.styles.colHeader.Render(truncateLine("  R A  WORKTREE            PROCESS         GROUP", maxTextWidth)))
	statusByWorktree := map[string]runtime.StatusRow{}
	for _, row := range m.rows {
		statusByWorktree[row.Worktree] = row
	}

	for i, wt := range m.rc.repo.Worktrees {
		row := statusByWorktree[wt.Name]
		runIcon := "-"
		if row.Running {
			runIcon = "R"
		}
		activeIcon := "-"
		if row.Active {
			activeIcon = "A"
		}
		sel := " "
		if i == m.idx {
			sel = ">"
		}
		group := wt.Group
		if proc, err := m.rc.project.Process(wt.Process); err == nil {
			group = model.EffectiveGroup(proc, wt.Group, wt.Name)
		}
		plainLine := fmt.Sprintf(
			"%s %s %s  %-18s  %-14s  %-12s",
			sel,
			runIcon,
			activeIcon,
			truncateLine(wt.Name, 18),
			truncateLine(wt.Process, 14),
			truncateLine(group, 12),
		)
		plainLine = truncateLine(plainLine, maxTextWidth)
		line := plainLine
		if i == m.idx {
			line = m.styles.selectedRow.Render(plainLine)
		} else {
			line = m.styles.row.Render(plainLine)
		}
		lines = append(lines, line)
	}

	return m.renderPanel("Worktrees", lines, width, height, true)
}

func (m *tuiModel) renderDetailPanel(width, height int) string {
	maxTextWidth := max(8, width-4)
	wt := m.current()
	if wt == nil {
		return m.renderPanel("Details", []string{"No worktree selected."}, width, height, false)
	}

	proc, procErr := m.rc.project.Process(wt.Process)
	group := wt.Group
	if procErr == nil {
		group = model.EffectiveGroup(proc, wt.Group, wt.Name)
	}

	status := runtime.StatusRow{}
	for _, row := range m.rows {
		if row.Worktree == wt.Name {
			status = row
			break
		}
	}
	runBadge := m.styles.badgeStopped.Render("stopped")
	if status.Running {
		runBadge = m.styles.badgeRunning.Render("running")
	}
	activeBadge := m.styles.badgeStopped.Render("inactive")
	if status.Active {
		activeBadge = m.styles.badgeActive.Render("active")
	}

	lines := []string{
		m.styles.panelTitle.Render("status:") + " " + runBadge + " | " + activeBadge,
		"",
		m.kv("worktree", truncateLine(wt.Name, maxTextWidth-11)),
		m.kv("dir", truncateLine(wt.Dir, maxTextWidth-6)),
		m.kv("process", truncateLine(wt.Process, maxTextWidth-10)),
		m.kv("group", truncateLine(group, maxTextWidth-8)),
		"",
		m.styles.panelTitle.Render("Command"),
	}
	if procErr != nil {
		lines = append(lines, m.styles.error.Render(procErr.Error()))
	} else {
		for _, part := range wrapCommand(proc.Command, maxTextWidth) {
			lines = append(lines, m.styles.row.Render(part))
		}
	}
	return m.renderPanel("Selected", lines, width, height, false)
}

func (m *tuiModel) kv(k, v string) string {
	return m.styles.metaLabel.Render(k+":") + " " + m.styles.metaValue.Render(v)
}

func (m *tuiModel) renderPanel(title string, lines []string, width, height int, focused bool) string {
	innerHeight := max(1, height-2)

	content := make([]string, 0, innerHeight)
	titleLine := m.styles.panelTitle.Render(truncateLine(title, max(8, width-4)))
	content = append(content, titleLine)
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
		border = m.styles.panelBorderF
	}
	return border.Width(width).Render(strings.Join(content, "\n"))
}

func (m *tuiModel) renderFooter(width int) string {
	shortcuts := []string{
		m.keycap("n/p") + " next/prev",
		m.keycap("s") + " switch",
		m.keycap("r") + " restart",
		m.keycap("x") + " stop",
		m.keycap("a") + " add",
		m.keycap("d") + " remove",
		m.keycap("[") + "/" + m.keycap("]") + " process",
		m.keycap("g/u") + " group set/clear",
		m.keycap("q") + " quit",
	}
	line1 := truncateLine(strings.Join(shortcuts[:5], "   "), width)
	line2 := truncateLine(strings.Join(shortcuts[5:], "   "), width)

	msg := ""
	if m.message != "" {
		if strings.Contains(strings.ToLower(m.message), "error") || strings.Contains(strings.ToLower(m.message), "not found") {
			msg = m.styles.error.Render(truncateLine(m.message, width))
		} else {
			msg = m.styles.notice.Render(truncateLine(m.message, width))
		}
	}

	parts := []string{m.styles.footer.Render(line1), m.styles.footer.Render(line2)}
	if msg != "" {
		parts = append(parts, msg)
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(parts, "\n"))
}

func (m *tuiModel) keycap(s string) string {
	return m.styles.key.Render(s)
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

func wrapCommand(s string, width int) []string {
	if width < 8 {
		return []string{truncateLine(s, width)}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for i := 1; i < len(words); i++ {
		candidate := current + " " + words[i]
		if runeLen(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, truncateLine(current, width))
		current = words[i]
	}
	lines = append(lines, truncateLine(current, width))
	return lines
}

func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runeLen(s) <= width {
		return s
	}
	r := []rune(s)
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

func runeLen(s string) int {
	return len([]rune(s))
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
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
