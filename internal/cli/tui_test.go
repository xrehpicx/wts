package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
		styles: newTUIStyles(),
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
	worktrees := []gitwt.Worktree{{Name: "repo-main", Dir: "/tmp/repo-main", Branch: "main"}}
	backend := newTUITestBackend()
	window := tmux.WindowName("/tmp/repo-main")
	backend.windows[window] = true
	backend.panes[window] = []tmux.PaneInfo{
		{ID: "%0", Process: "api", Title: tmux.ProcessPaneTitle("api"), PID: "1000", Command: "node"},
		{ID: "%1", Process: "web", Title: tmux.ProcessPaneTitle("web"), PID: "1001", Command: "node"},
	}

	rc := &runtimeContext{
		project:   project,
		repoRoot:  "/tmp/repo",
		worktrees: worktrees,
		manager:   runtime.NewManager(project, "/tmp/repo", worktrees, backend),
	}
	m := newTUIModel(rc)
	m.selectTarget(model.Target{Kind: model.TargetProcess, Name: "web", ProcessNames: []string{"web"}})

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
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
