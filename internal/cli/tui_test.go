package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

type tuiTestBackend struct {
	windows map[string]bool
	options map[string]string
	panes   map[string][]tmux.PaneInfo
}

func newTUITestBackend() *tuiTestBackend {
	return &tuiTestBackend{
		windows: map[string]bool{},
		options: map[string]string{},
		panes:   map[string][]tmux.PaneInfo{},
	}
}

func (b *tuiTestBackend) EnsureTmux(context.Context) error { return nil }
func (b *tuiTestBackend) EnsureSession(context.Context, string) error {
	return nil
}
func (b *tuiTestBackend) HasWindow(_ context.Context, _ string, window string) (bool, error) {
	return b.windows[window], nil
}
func (b *tuiTestBackend) FindWindowSessions(context.Context, string) ([]string, error) {
	return nil, nil
}
func (b *tuiTestBackend) StartWindowCommand(context.Context, string, string, string, string, string, map[string]string, string) error {
	return nil
}
func (b *tuiTestBackend) StopWindow(context.Context, string, string, time.Duration) error {
	return nil
}
func (b *tuiTestBackend) SetSessionOption(_ context.Context, _ string, key, value string) error {
	if value == "" {
		delete(b.options, key)
		return nil
	}
	b.options[key] = value
	return nil
}
func (b *tuiTestBackend) GetSessionOption(_ context.Context, _ string, key string) (string, error) {
	return b.options[key], nil
}
func (b *tuiTestBackend) CapturePane(context.Context, string, string, int) (string, error) {
	return "", nil
}
func (b *tuiTestBackend) PaneCurrentCommand(context.Context, string, string) (string, error) {
	return "", nil
}
func (b *tuiTestBackend) Attach(context.Context, string, string, string) error { return nil }
func (b *tuiTestBackend) SetPaneTitle(context.Context, string, string, string) error {
	return nil
}
func (b *tuiTestBackend) SplitWindowCommand(context.Context, string, string, string, string, string, map[string]string, string) error {
	return nil
}
func (b *tuiTestBackend) ListPanes(_ context.Context, _, window string) ([]tmux.PaneInfo, error) {
	return b.panes[window], nil
}
func (b *tuiTestBackend) StopPane(context.Context, string, time.Duration) error { return nil }
func (b *tuiTestBackend) CapturePaneByID(context.Context, string, int) (string, error) {
	return "", nil
}
func (b *tuiTestBackend) PaneExitedByPID(context.Context, string) bool { return false }

func testTUIProject() *model.Project {
	return model.NewProject("", "/tmp/repo", model.Config{
		Version: model.CurrentVersion,
		Processes: []model.Process{
			{Name: "api", Command: "go run ."},
			{Name: "web", Command: "pnpm dev"},
		},
		Groups: []model.ProcessGroup{
			{Name: "dev", Processes: []string{"api", "web"}},
		},
	})
}

func runCmdMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	msgs := make([]tea.Msg, 0, len(batch))
	for _, nested := range batch {
		if nested == nil {
			continue
		}
		msgs = append(msgs, nested())
	}
	return msgs
}

func TestFormatTargetLabelForGroup(t *testing.T) {
	t.Parallel()

	label := formatTargetLabel(model.Target{Kind: model.TargetGroup, Name: "dev"})
	if label != "[group] dev" {
		t.Fatalf("unexpected label: %q", label)
	}
}

func TestEnterCreateGroupModePreselectsSelectedTargetMembers(t *testing.T) {
	t.Parallel()

	project := model.NewProject("", "/tmp", model.Config{
		Version: model.CurrentVersion,
		Processes: []model.Process{
			{Name: "api", Command: "go run ."},
			{Name: "web", Command: "pnpm dev"},
		},
	})

	m := &tuiModel{
		rc:                  &runtimeContext{project: project},
		targets:             project.Targets(),
		targetIdx:           0,
		createGroupInput:    textinput.New(),
		createGroupSelected: map[string]bool{},
	}

	m.enterCreateGroupMode()

	if !m.createGroupMode {
		t.Fatal("expected create group mode enabled")
	}
	if !m.createGroupSelected["api"] {
		t.Fatal("expected selected process preselected in group editor")
	}
	if m.createGroupSelected["web"] {
		t.Fatal("did not expect unselected process preselected")
	}
}

func TestSelectedCreateGroupMembersPreservesProcessOrder(t *testing.T) {
	t.Parallel()

	project := model.NewProject("", "/tmp", model.Config{
		Version: model.CurrentVersion,
		Processes: []model.Process{
			{Name: "api", Command: "go run ."},
			{Name: "web", Command: "pnpm dev"},
			{Name: "worker", Command: "go run ./cmd/worker"},
		},
	})

	m := &tuiModel{
		rc: &runtimeContext{project: project},
		createGroupSelected: map[string]bool{
			"worker": true,
			"api":    true,
		},
	}

	members := m.selectedCreateGroupMembers()
	if len(members) != 2 {
		t.Fatalf("unexpected member count: %d", len(members))
	}
	if members[0] != "api" || members[1] != "worker" {
		t.Fatalf("unexpected member order: %#v", members)
	}
}

func TestRenderGroupLogsKeepsProcessesSeparate(t *testing.T) {
	t.Parallel()

	m := &tuiModel{
		logLines: map[string][]string{
			"test-loop":   {"test-1", "test-2"},
			"demo-script": {"demo-1", "demo-2"},
		},
		styles: newTUIStyles(true),
	}
	target := model.Target{
		Kind:         model.TargetGroup,
		Name:         "dev",
		ProcessNames: []string{"test-loop", "demo-script"},
	}

	lines := m.renderGroupLogs(target, 6, 80)
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "test-loop") || !strings.Contains(rendered, "demo-script") {
		t.Fatalf("expected both process names in group log output, got %q", rendered)
	}
	if !strings.Contains(rendered, "test-1") || !strings.Contains(rendered, "demo-1") {
		t.Fatalf("expected log lines from both processes, got %q", rendered)
	}
}

func TestAttachKeyResolvesSelectedProcessPane(t *testing.T) {
	t.Parallel()

	project := testTUIProject()
	repoRoot := t.TempDir()
	worktreeDir := filepath.Join(repoRoot, "repo-main")
	worktrees := []gitwt.Worktree{{Name: "repo-main", Dir: worktreeDir, Branch: "main"}}
	backend := newTUITestBackend()
	window := tmux.WindowName(worktreeDir)
	backend.windows[window] = true
	backend.panes[window] = []tmux.PaneInfo{
		{ID: "%0", Process: "api", Title: tmux.ProcessPaneTitle("api"), PID: "1000", Command: "node"},
		{ID: "%1", Process: "web", Title: tmux.ProcessPaneTitle("web"), PID: "1001", Command: "node"},
	}

	rc := &runtimeContext{
		project:   project,
		repoRoot:  repoRoot,
		worktrees: worktrees,
		manager:   runtime.NewManager(project, repoRoot, worktrees, backend),
	}
	m := newTUIModel(rc)
	m.selectTarget(model.Target{Kind: model.TargetProcess, Name: "web", ProcessNames: []string{"web"}})

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	updated := updatedModel.(*tuiModel)
	if !updated.loading {
		t.Fatal("expected attach action to enter loading state")
	}

	msgs := runCmdMessages(cmd)
	if len(msgs) != 2 {
		t.Fatalf("expected spinner tick and attach resolution, got %d messages", len(msgs))
	}

	var attachMsg attachReadyMsg
	found := false
	for _, msg := range msgs {
		if candidate, ok := msg.(attachReadyMsg); ok {
			attachMsg = candidate
			found = true
		}
	}
	if !found {
		t.Fatalf("expected attachReadyMsg, got %#v", msgs)
	}
	if attachMsg.spec.Window != window {
		t.Fatalf("unexpected attach window: %q", attachMsg.spec.Window)
	}
	if attachMsg.spec.PaneID != "%1" {
		t.Fatalf("expected web pane selected, got %q", attachMsg.spec.PaneID)
	}
}

func TestNewTUIModelSelectsFirstTargetByDefault(t *testing.T) {
	t.Parallel()

	project := testTUIProject()
	worktrees := []gitwt.Worktree{{Name: "repo-main", Dir: "/tmp/repo-main", Branch: "main"}}
	backend := newTUITestBackend()
	m := newTUIModel(&runtimeContext{
		project:   project,
		repoRoot:  "/tmp/repo",
		worktrees: worktrees,
		manager:   runtime.NewManager(project, "/tmp/repo", worktrees, backend),
	})

	target, ok := m.selectedTarget()
	if !ok || target.Name != "api" {
		t.Fatalf("expected first target selected, got %#v, selected=%v", target, ok)
	}
}

func TestTUIViewStaysWithinTerminalBounds(t *testing.T) {
	t.Parallel()

	project := testTUIProject()
	worktrees := []gitwt.Worktree{
		{Name: "repo-main-with-a-long-name", Dir: "/tmp/repo-main", Branch: "feature/a-very-long-branch-name"},
		{Name: "repo-agent", Dir: "/tmp/repo-agent", Branch: "agent"},
	}
	backend := newTUITestBackend()
	m := newTUIModel(&runtimeContext{
		project:   project,
		repoRoot:  "/tmp/a-repository-with-a-long-name",
		worktrees: worktrees,
		manager:   runtime.NewManager(project, "/tmp/repo", worktrees, backend),
	})

	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 48, height: 20},
		{width: 24, height: 4},
		{width: 72, height: 24},
		{width: 120, height: 32},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m.width = size.width
			m.height = size.height
			content := m.View().Content
			if got := lipgloss.Width(content); got > size.width {
				t.Fatalf("view width = %d; terminal width = %d", got, size.width)
			}
			if got := lipgloss.Height(content); got > size.height {
				t.Fatalf(
					"view height = %d; terminal height = %d (header=%d content=%d footer=%d left=%d right=%d)",
					got,
					size.height,
					lipgloss.Height(m.renderHeader(size.width)),
					lipgloss.Height(m.renderContent(size.width, size.height-lipgloss.Height(m.renderHeader(size.width))-lipgloss.Height(m.renderFooter(size.width)))),
					lipgloss.Height(m.renderFooter(size.width)),
					lipgloss.Height(m.renderListPanel((size.width-1)/3, size.height-lipgloss.Height(m.renderHeader(size.width))-lipgloss.Height(m.renderFooter(size.width)))),
					lipgloss.Height(m.renderDetailPanel((size.width-1)-((size.width-1)/3), size.height-lipgloss.Height(m.renderHeader(size.width))-lipgloss.Height(m.renderFooter(size.width)))),
				)
			}
		})
	}
}

func TestWorktreeListScrollsToKeepSelectionVisible(t *testing.T) {
	t.Parallel()

	rows := make([]runtime.StatusRow, 12)
	worktrees := make([]gitwt.Worktree, len(rows))
	for i := range rows {
		name := fmt.Sprintf("worktree-%02d", i)
		dir := filepath.Join("/tmp", name)
		rows[i] = runtime.StatusRow{Worktree: name, Dir: dir, Branch: "main"}
		worktrees[i] = gitwt.Worktree{Name: name, Dir: dir, Branch: "main"}
	}

	project := testTUIProject()
	m := newTUIModel(&runtimeContext{
		project:   project,
		repoRoot:  "/tmp/repo",
		worktrees: worktrees,
		manager:   runtime.NewManager(project, "/tmp/repo", worktrees, newTUITestBackend()),
	})
	m.rows = rows
	m.idx = 8

	panel := m.renderListPanel(48, 12)
	if !strings.Contains(panel, "▸ ") || !strings.Contains(panel, "worktree-08") {
		t.Fatalf("selected worktree was rendered outside the viewport:\n%s", panel)
	}
	if strings.Contains(panel, "worktree-00") {
		t.Fatalf("viewport did not scroll away from the first worktree:\n%s", panel)
	}
	if m.listOffset == 0 {
		t.Fatal("expected list viewport offset to advance")
	}

	m.idx = 0
	panel = m.renderListPanel(48, 12)
	if !strings.Contains(panel, "▸ ") || !strings.Contains(panel, "worktree-00") || m.listOffset != 0 {
		t.Fatalf("viewport did not follow wrapped selection back to the start:\n%s", panel)
	}
}

func TestCreateGroupViewHandlesTinyTerminal(t *testing.T) {
	t.Parallel()

	m := newTUIModel(&runtimeContext{
		project:  testTUIProject(),
		repoRoot: "/tmp/repo",
		manager:  runtime.NewManager(testTUIProject(), "/tmp/repo", nil, newTUITestBackend()),
	})
	m.createGroupMode = true

	for height := 1; height <= 4; height++ {
		got := m.renderCreateGroupPanel(20, height)
		if lipgloss.Width(got) > 20 || lipgloss.Height(got) > height {
			t.Fatalf("height %d rendered outside bounds: %dx%d", height, lipgloss.Width(got), lipgloss.Height(got))
		}
	}
}

func TestTargetProcessStateFallsBackToTargetName(t *testing.T) {
	t.Parallel()

	row := &runtime.StatusRow{Processes: []runtime.ProcessStatus{{Name: "api", Running: true}}}
	managed, exited := targetProcessState(row, model.Target{Name: "api"})
	if !managed || exited {
		t.Fatalf("target state = managed %v, exited %v; want true, false", managed, exited)
	}
}

func TestFilterClearsSelectionWhenNothingMatches(t *testing.T) {
	t.Parallel()

	m := &tuiModel{
		targets:   testTUIProject().Targets(),
		targetIdx: 0,
	}
	m.filterProcesses("does-not-exist")
	if _, ok := m.selectedTarget(); ok {
		t.Fatal("expected no selected target when the filter has no matches")
	}
}

func TestTruncateLineHandlesANSIAndWideCharacters(t *testing.T) {
	t.Parallel()

	styled := lipgloss.NewStyle().Bold(true).Render("界界界")
	got := truncateLine(styled, 4)
	if width := lipgloss.Width(got); width > 4 {
		t.Fatalf("truncated width = %d; want <= 4; output %q", width, got)
	}
}
