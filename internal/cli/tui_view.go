package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/xrehpicx/wts/internal/model"
)

func (m *tuiModel) View() tea.View {
	w, h := m.width, m.height
	if w <= 0 {
		w = 110
	}
	if h <= 0 {
		h = 30
	}
	var content string
	if h < 9 || w < 16 {
		lines := []string{" wts · " + filepath.Base(m.rc.repoRoot)}
		if m.loading {
			lines = append(lines, m.loadingMsg)
		} else if m.message != "" {
			lines = append(lines, m.message)
		}
		if row := m.current(); row != nil {
			lines = append(lines, "Selected: "+row.Worktree)
		}
		if target, ok := m.selectedTarget(); ok {
			lines = append(lines, formatTargetLabel(target))
		}
		lines = append(lines, "Resize for full view · q quit")
		content = fitBlock(strings.Join(lines, "\n"), w, h)
	} else {
		header, footer := m.renderHeader(w), m.renderFooter(w)
		contentH := max(1, h-lipgloss.Height(header)-lipgloss.Height(footer))
		content = lipgloss.JoinVertical(lipgloss.Left, header, m.renderContent(w, contentH), footer)
	}
	view := tea.NewView(fitBlock(content, w, h))
	view.AltScreen = true
	view.WindowTitle = "wts · " + filepath.Base(m.rc.repoRoot)
	return view
}

// Feedback has its own row so a long target name never hides an error.
func (m *tuiModel) renderHeader(width int) string {
	left := " " + m.styles.title.Render("wts") + m.styles.subtitle.Render(" · "+filepath.Base(m.rc.repoRoot))
	right := m.styles.subtitle.Render(fmt.Sprintf("%d worktrees", len(m.rows))) + " "
	lines := []string{headerRow(left, right, width)}
	selected := "No worktree selected"
	if row := m.current(); row != nil {
		selected = row.Worktree
	}
	targetLabel := "No target"
	if target, ok := m.selectedTarget(); ok {
		targetLabel = formatTargetLabel(target)
	}
	lines = append(lines, truncateLine(" "+m.styles.metaValue.Render(selected)+m.styles.dimText.Render(" / ")+m.styles.title.Render(targetLabel), width))
	if m.loading {
		lines = append(lines, truncateLine(" "+m.styles.statusBusy.Render(m.spinner.View()+" "+m.loadingMsg), width))
	} else if m.message != "" {
		style := m.styles.statusOk
		prefix := "✓ "
		if m.messageIsErr {
			style = m.styles.statusErr
			prefix = "✗ "
		}
		lines = append(lines, truncateLine(" "+style.Render(prefix+m.message), width))
	}
	lines = append(lines, m.styles.separator.Render(strings.Repeat("─", max(0, width))))
	return strings.Join(lines, "\n")
}

func (m *tuiModel) renderContent(width, height int) string {
	width = max(1, width)
	height = max(1, height)
	if m.createGroupMode {
		return m.renderCreateGroupPanel(width, height)
	}
	if m.filterMode {
		return m.renderTargetSearch(width, height)
	}
	if m.showAll {
		return m.renderHelpPanel(width, height)
	}
	if len(m.rows) == 0 {
		empty := m.styles.dimText.Render("No worktrees found. Create one with:")
		hint := m.styles.metaValue.Render("  git worktree add ../branch-name")
		return m.renderPanel("Worktrees", []string{empty, hint}, width, height, true)
	}

	if width < 96 {
		if height < 8 {
			return m.renderDetailPanel(width, height)
		}
		usableHeight := height
		listHeight := (usableHeight * 2) / 5
		listHeight = min(max(8, listHeight), usableHeight-3)
		detailHeight := usableHeight - listHeight
		if detailHeight < 3 {
			detailHeight = 3
			listHeight = max(3, usableHeight-detailHeight)
		}
		left := m.renderCompactListPanel(width, listHeight)
		right := m.renderDetailPanel(width, detailHeight)
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	spacer := 1
	usableWidth := max(2, width-spacer)
	leftWidth := min(46, max(30, usableWidth/3))
	rightWidth := usableWidth - leftWidth
	left := m.renderListPanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	left = lipgloss.NewStyle().MarginRight(spacer).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *tuiModel) renderListPanel(width, height int) string {
	maxTextWidth := max(1, width-m.styles.panelFocus.GetHorizontalFrameSize())
	innerHeight := max(1, height-m.styles.panelFocus.GetVerticalFrameSize())
	lineCapacity := max(1, innerHeight-2) // panel title and spacer
	start, end := visibleWorktreeRange(len(m.rows), m.idx, m.listOffset, lineCapacity)
	m.listOffset = start
	lines := make([]string, 0, (end-start)*3)

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

	for i := start; i < end; i++ {
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
			line1 = headerRow(namePart, m.styles.dimText.Render(procBadge), maxTextWidth)
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
			line1 = sel.Render(truncateLine(ansi.Strip(line1), maxTextWidth))
			line2 = sel.Render(truncateLine(ansi.Strip(line2), maxTextWidth))
		}

		lines = append(lines, line1, line2)

		// Add a blank separator between visible entries.
		if i < end-1 {
			lines = append(lines, "")
		}
	}

	title := fmt.Sprintf("Worktrees · %d/%d", m.idx+1, len(m.rows))
	return m.renderPanel(title, lines, width, height, true)
}

// Compact rows keep several worktrees visible in stacked terminal layouts.
func (m *tuiModel) renderCompactListPanel(width, height int) string {
	capacity := max(1, height-4)
	start := min(m.listOffset, max(0, len(m.rows)-capacity))
	if m.idx < start {
		start = m.idx
	}
	if m.idx >= start+capacity {
		start = m.idx - capacity + 1
	}
	m.listOffset = max(0, start)
	maxW := max(1, width-m.styles.panelBorder.GetHorizontalFrameSize())
	lines := make([]string, 0, capacity)
	for i := m.listOffset; i < min(len(m.rows), m.listOffset+capacity); i++ {
		row := m.rows[i]
		prefix := "  "
		if i == m.idx {
			prefix = "▸ "
		}
		state := "○"
		if row.Prunable {
			state = "!"
		} else if row.Running && row.Exited {
			state = "exited"
		} else if row.Running {
			state = "●"
		}
		name := row.Worktree
		if row.Active {
			name += " ★"
		}
		line := headerRow(prefix+name, state, maxW)
		style := m.styles.row
		if i == m.idx {
			style = m.styles.selectedRow.Width(maxW)
		}
		lines = append(lines, style.Render(truncateLine(line, maxW)))
	}
	return m.renderPanel(fmt.Sprintf("Worktrees · %d/%d", m.idx+1, len(m.rows)), lines, width, height, true)
}

func visibleWorktreeRange(total, selected, offset, lineCapacity int) (int, int) {
	if total <= 0 {
		return 0, 0
	}

	// Each worktree uses two content lines plus one separator. The final
	// visible entry does not need a separator, hence the extra line here.
	visible := max(1, (max(1, lineCapacity)+1)/3)
	visible = min(visible, total)
	selected = min(max(0, selected), total-1)
	offset = min(max(0, offset), total-visible)

	if selected < offset {
		offset = selected
	} else if selected >= offset+visible {
		offset = selected - visible + 1
	}

	return offset, min(total, offset+visible)
}

func (m *tuiModel) renderDetailPanel(width, height int) string {
	maxW := max(1, width-m.styles.panelBorder.GetHorizontalFrameSize())
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
	if ok {
		label = " " + target.Name + " "
	}
	sepW := max(0, maxW-lipgloss.Width(label))
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
	targetManaged, targetExited := targetProcessState(row, target)
	if targetManaged && targetExited {
		hint = m.styles.dimText.Render("a attach tmux · r restart · x stop · " + targetNoun + " exited")
	} else if targetManaged {
		hint = m.styles.dimText.Render("a attach tmux · r restart · x stop " + targetNoun)
	} else if row.Running {
		hint = m.styles.dimText.Render("s/↵ add " + targetNoun)
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

		}
		if len(lines) < capacity-1 && (len(m.logLines) == 0 || !targetManaged || (target.Kind != model.TargetGroup && len(m.logLines[target.Name]) == 0)) {
			empty := "Waiting for output…"
			if !targetManaged {
				empty = "Press enter to start this " + targetNoun
			}
			lines = append(lines, m.styles.dimText.Render(empty))
		}
	}

	for len(lines) < capacity-1 {
		lines = append(lines, "")
	}
	lines = append(lines, hint)

	return m.renderPanel(panelTitle, lines, width, height, false)
}

func (m *tuiModel) renderCreateGroupPanel(width, height int) string {
	if width < 3 || height < 3 {
		return truncateLine("Create Group", max(1, width))
	}
	maxW := max(1, width-m.styles.modalBorder.GetHorizontalFrameSize())
	innerHeight := max(1, height-2)
	capacity := max(0, innerHeight-2)
	if capacity == 0 {
		return renderBordered(m.styles.modalBorder,
			[]string{m.styles.panelTitle.Render("Create Group")}, width, height)
	}

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
		memberCapacity := max(1, capacity-len(lines))
		start := 0
		if m.createGroupFocus == createGroupFocusMembers && m.createGroupCursor >= memberCapacity {
			start = m.createGroupCursor - memberCapacity + 1
		}
		end := min(len(processNames), start+memberCapacity)
		for i := start; i < end; i++ {
			name := processNames[i]
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

	if len(lines) > capacity {
		lines = lines[:capacity]
	}
	for len(lines) < capacity {
		lines = append(lines, "")
	}

	content := append([]string{m.styles.panelTitle.Render("Create Group"), ""}, lines...)
	return renderBordered(m.styles.modalBorder, content, width, height)
}

func (m *tuiModel) renderCreateGroupNameLine(maxW int) string {
	m.createGroupInput.SetWidth(max(1, maxW-1))
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
	border := m.styles.panelBorder
	if focused {
		border = m.styles.panelFocus
	}
	innerWidth := max(1, width-border.GetHorizontalFrameSize())
	innerHeight := max(1, height-border.GetVerticalFrameSize())

	content := make([]string, 0, innerHeight)
	content = append(content, m.styles.panelTitle.Render(truncateLine(title, innerWidth)))
	content = append(content, "")
	for _, line := range lines {
		content = append(content, truncateLine(line, innerWidth))
		if len(content) >= innerHeight {
			break
		}
	}
	for len(content) < innerHeight {
		content = append(content, "")
	}

	return renderBordered(border, content, width, height)
}

func renderBordered(style lipgloss.Style, lines []string, width, height int) string {
	if width <= style.GetHorizontalFrameSize() || height <= style.GetVerticalFrameSize() {
		return fitBlock(strings.Join(lines, "\n"), width, height)
	}
	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	innerHeight := max(1, height-style.GetVerticalFrameSize())
	content := make([]string, 0, innerHeight)
	for _, line := range lines {
		if len(content) >= innerHeight {
			break
		}
		content = append(content, truncateLine(line, innerWidth))
	}
	for len(content) < innerHeight {
		content = append(content, "")
	}
	return style.Width(max(1, width)).Render(strings.Join(content, "\n"))
}

func (m *tuiModel) renderFooter(width int) string {
	text := "↑↓ worktree · ←→ target · enter start · / search · g group · ? help · q quit"
	switch {
	case m.loading:
		text = "Working… · ctrl+c quit"
	case m.createGroupMode:
		text = "tab focus · ↑↓ move · space toggle · enter save · esc cancel"
		if width < 72 {
			text = "tab focus · space toggle · ↵ save · esc cancel"
		}
	case m.filterMode:
		text = "↑↓ match · enter select · esc cancel"
	case m.showAll:
		text = "? close help · q quit"
	case width < 72:
		text = "↑↓ tree · ←→ target · ↵ start · ? help · q quit"
	}
	return truncateLine(" "+m.styles.footer.Render(text), width)
}

func (m *tuiModel) renderHelpPanel(width, height int) string {
	lines := []string{}
	for _, column := range m.keys.FullHelp() {
		for _, binding := range column {
			h := binding.Help()
			lines = append(lines, fmt.Sprintf("%-8s %s", h.Key, h.Desc))
		}
	}
	// Two balanced columns retain all shortcuts on ordinary 80x24 terminals.
	if width >= 60 {
		n := (len(lines) + 1) / 2
		paired := make([]string, 0, n)
		for i := 0; i < n; i++ {
			line := lines[i]
			if i+n < len(lines) {
				line = lipgloss.NewStyle().Width((width-4)/2).Render(line) + lines[i+n]
			}
			paired = append(paired, line)
		}
		lines = paired
	}
	return m.renderPanel("Keyboard shortcuts", lines, width, height, false)
}

func (m *tuiModel) renderTargetSearch(width, height int) string {
	maxW := max(1, width-m.styles.panelFocus.GetHorizontalFrameSize())
	m.filterInput.SetWidth(max(1, maxW-3))
	lines := []string{"/ " + m.filterInput.View(), ""}
	var matches []int
	selected := 0
	query := strings.ToLower(m.filterInput.Value())
	for i, target := range m.targets {
		if strings.Contains(strings.ToLower(formatTargetLabel(target)), query) {
			if i == m.targetIdx {
				selected = len(matches)
			}
			matches = append(matches, i)
		}
	}
	capacity := max(1, height-6)
	start := max(0, selected-capacity+1)
	for _, i := range matches[start:min(len(matches), start+capacity)] {
		prefix := "  "
		style := m.styles.row
		if i == m.targetIdx {
			prefix = "▸ "
			style = m.styles.selectedRow
		}
		lines = append(lines, style.Render(truncateLine(prefix+formatTargetLabel(m.targets[i]), maxW)))
	}
	if len(matches) == 0 {
		lines = append(lines, m.styles.dimText.Render("No matching targets. Try another name."))
	}
	return m.renderPanel(fmt.Sprintf("Search targets · %d matches", len(matches)), lines, width, height, true)
}

// fitBlock is a final guard for terminal dimensions, including very small sizes.
func fitBlock(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = truncateLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// --- Helpers ---

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	relative, err := filepath.Rel(home, p)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return p
	}
	if relative == "." {
		return "~"
	}
	return filepath.Join("~", relative)
}

func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = strings.NewReplacer("\n", "↵", "\r", "", "\t", " ").Replace(s)
	return ansi.Truncate(s, width, "…")
}

func headerRow(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap >= 1 {
		return left + strings.Repeat(" ", gap) + right
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth+1 >= width {
		return truncateLine(left, width)
	}
	left = truncateLine(left, width-rightWidth-1)
	return left + " " + right
}
