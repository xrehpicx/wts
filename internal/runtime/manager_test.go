package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/tmux"
)

type mockBackend struct {
	windows     map[string]bool
	options     map[string]string
	startCount  map[string]int
	stopCount   map[string]int
	panes       map[string][]tmux.PaneInfo // window -> panes
	exitedByPID map[string]bool
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		windows:     map[string]bool{},
		options:     map[string]string{},
		startCount:  map[string]int{},
		stopCount:   map[string]int{},
		panes:       map[string][]tmux.PaneInfo{},
		exitedByPID: map[string]bool{},
	}
}

func (m *mockBackend) EnsureTmux(context.Context) error { return nil }
func (m *mockBackend) EnsureSession(context.Context, string) error {
	return nil
}
func (m *mockBackend) HasWindow(_ context.Context, _ string, window string) (bool, error) {
	return m.windows[window], nil
}
func (m *mockBackend) StartWindowCommand(_ context.Context, _ string, window, _ string, _ string, _ string, _ map[string]string) error {
	m.windows[window] = true
	m.startCount[window]++
	return nil
}
func (m *mockBackend) StopWindow(_ context.Context, _ string, window string, _ time.Duration) error {
	m.windows[window] = false
	m.stopCount[window]++
	delete(m.panes, window)
	return nil
}
func (m *mockBackend) SetSessionOption(_ context.Context, _ string, key, value string) error {
	if value == "" {
		delete(m.options, key)
		return nil
	}
	m.options[key] = value
	return nil
}
func (m *mockBackend) GetSessionOption(_ context.Context, _ string, key string) (string, error) {
	return m.options[key], nil
}
func (m *mockBackend) CapturePane(context.Context, string, string, int) (string, error) {
	return "", nil
}
func (m *mockBackend) PaneCurrentCommand(context.Context, string, string) (string, error) {
	return "", nil
}
func (m *mockBackend) Attach(context.Context, string, string) error { return nil }

func (m *mockBackend) SetPaneTitle(_ context.Context, _, window, title string) error {
	if len(m.panes[window]) > 0 {
		m.panes[window][len(m.panes[window])-1].Title = title
		m.panes[window][len(m.panes[window])-1].Process = tmux.ProcessFromPaneTitle(title)
	} else {
		m.panes[window] = []tmux.PaneInfo{{ID: "%0", Process: tmux.ProcessFromPaneTitle(title), Title: title, PID: "1000", Command: "node"}}
	}
	return nil
}
func (m *mockBackend) SplitWindowCommand(_ context.Context, _, window, _, _, _ string, _ map[string]string, paneTitle string) error {
	id := len(m.panes[window])
	m.panes[window] = append(m.panes[window], tmux.PaneInfo{
		ID:      "%" + string(rune('0'+id)),
		Process: tmux.ProcessFromPaneTitle(paneTitle),
		Title:   paneTitle,
		PID:     "100" + string(rune('0'+id)),
		Command: "node",
	})
	m.startCount[window]++
	return nil
}
func (m *mockBackend) ListPanes(_ context.Context, _, window string) ([]tmux.PaneInfo, error) {
	return m.panes[window], nil
}
func (m *mockBackend) StopPane(_ context.Context, paneID string, _ time.Duration) error {
	for w, panes := range m.panes {
		for i, p := range panes {
			if p.ID == paneID {
				m.panes[w] = append(panes[:i], panes[i+1:]...)
				if len(m.panes[w]) == 0 {
					delete(m.panes, w)
					m.windows[w] = false
				}
				return nil
			}
		}
	}
	return nil
}
func (m *mockBackend) CapturePaneByID(context.Context, string, int) (string, error) {
	return "", nil
}
func (m *mockBackend) PaneExitedByPID(_ context.Context, pid string) bool {
	return m.exitedByPID[pid]
}

func testProject() *model.Project {
	cfg := model.Config{
		Version: model.CurrentVersion,
		Defaults: model.Defaults{
			StopTimeoutSec: model.DefaultStopTimeout,
			Shell:          model.DefaultShell,
		},
		Processes: []model.Process{
			{Name: "api", Command: "go run .", Env: map[string]string{}},
			{Name: "web", Command: "pnpm dev", Env: map[string]string{}},
		},
		Groups: []model.ProcessGroup{
			{Name: "dev", Processes: []string{"api", "web"}},
		},
	}
	return model.NewProject("/tmp/.wts.yaml", "/tmp", cfg)
}

func testWorktrees() []gitwt.Worktree {
	return []gitwt.Worktree{
		{Name: "repo-main", Dir: "/tmp/repo-main", Branch: "main"},
		{Name: "repo-agent", Dir: "/tmp/repo-agent", Branch: "agent"},
	}
}

func TestSwitchStartsWorktreeAndMarksActive(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)

	if err := manager.Switch(context.Background(), "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	if !backend.windows[window] {
		t.Fatalf("expected %q to be running", window)
	}
	if backend.options[tmux.ActiveWorktreeOptionKey()] != "/tmp/repo-main" {
		t.Fatalf("unexpected active worktree: %q", backend.options[tmux.ActiveWorktreeOptionKey()])
	}
	if backend.options[tmux.ActiveProcessOptionKey()] != "api" {
		t.Fatalf("unexpected active process: %q", backend.options[tmux.ActiveProcessOptionKey()])
	}
}

func TestSwitchPreemptsPreviousWorktree(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch repo-main: %v", err)
	}
	if err := manager.Switch(ctx, "repo-agent", RunOptions{}); err != nil {
		t.Fatalf("switch repo-agent: %v", err)
	}

	if backend.windows[tmux.WindowName("/tmp/repo-main")] {
		t.Fatalf("expected previous worktree stopped")
	}
	if !backend.windows[tmux.WindowName("/tmp/repo-agent")] {
		t.Fatalf("expected target worktree running")
	}
}

func TestSwitchUsesRequestedProcess(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)

	if err := manager.Switch(context.Background(), "repo-agent", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	if backend.options[tmux.ActiveProcessOptionKey()] != "web" {
		t.Fatalf("unexpected active process: %q", backend.options[tmux.ActiveProcessOptionKey()])
	}
	if backend.options[tmux.ActiveTargetKindOptionKey()] != string(model.TargetProcess) {
		t.Fatalf("unexpected active target kind: %q", backend.options[tmux.ActiveTargetKindOptionKey()])
	}
	if backend.options[tmux.ActiveTargetNameOptionKey()] != "web" {
		t.Fatalf("unexpected active target name: %q", backend.options[tmux.ActiveTargetNameOptionKey()])
	}
}

func TestStartIsAdditive(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	// Start api in repo-main.
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	// Start web in same worktree (should add a pane, not replace).
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	panes := backend.panes[window]
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	if tmux.ProcessFromPaneTitle(panes[0].Title) != "api" {
		t.Fatalf("expected first pane to be api, got %q", panes[0].Title)
	}
	if tmux.ProcessFromPaneTitle(panes[1].Title) != "web" {
		t.Fatalf("expected second pane to be web, got %q", panes[1].Title)
	}
}

func TestStartGroupStartsAllProcesses(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start group: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	panes := backend.panes[window]
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes for group, got %d", len(panes))
	}
	if backend.options[tmux.ActiveTargetKindOptionKey()] != string(model.TargetGroup) {
		t.Fatalf("unexpected active target kind: %q", backend.options[tmux.ActiveTargetKindOptionKey()])
	}
	if backend.options[tmux.ActiveTargetNameOptionKey()] != "dev" {
		t.Fatalf("unexpected active target name: %q", backend.options[tmux.ActiveTargetNameOptionKey()])
	}
}

func TestStopProcessStopsOnlyOnePane(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}
	if err := manager.StopProcess(ctx, "repo-main", "api"); err != nil {
		t.Fatalf("stop api: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	panes := backend.panes[window]
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane after stopping api, got %d", len(panes))
	}
	if tmux.ProcessFromPaneTitle(panes[0].Title) != "web" {
		t.Fatalf("expected remaining pane to be web, got %q", panes[0].Title)
	}
}

func TestStopGroupStopsAllGroupPanes(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start group: %v", err)
	}
	if err := manager.StopGroup(ctx, "repo-main", "dev"); err != nil {
		t.Fatalf("stop group: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	if backend.windows[window] {
		t.Fatalf("expected worktree window stopped after stopping entire group")
	}
	if backend.options[tmux.ActiveTargetNameOptionKey()] != "" {
		t.Fatalf("expected active target cleared")
	}
}

func TestStopActiveClearsActiveOptions(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if err := manager.StopActive(ctx); err != nil {
		t.Fatalf("stop active: %v", err)
	}
	if backend.options[tmux.ActiveWorktreeOptionKey()] != "" {
		t.Fatalf("expected active worktree cleared")
	}
	if backend.options[tmux.ActiveProcessOptionKey()] != "" {
		t.Fatalf("expected active process cleared")
	}
	if backend.options[tmux.ActiveTargetKindOptionKey()] != "" {
		t.Fatalf("expected active target kind cleared")
	}
	if backend.options[tmux.ActiveTargetNameOptionKey()] != "" {
		t.Fatalf("expected active target name cleared")
	}
}

func TestStartDoesNotPreemptOtherWorktrees(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	// Start api in repo-main.
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start repo-main: %v", err)
	}
	// Start web in repo-agent — should NOT stop repo-main.
	if err := manager.Start(ctx, "repo-agent", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start repo-agent: %v", err)
	}

	winMain := tmux.WindowName("/tmp/repo-main")
	winAgent := tmux.WindowName("/tmp/repo-agent")
	if !backend.windows[winMain] {
		t.Fatalf("expected repo-main still running after Start on repo-agent")
	}
	if !backend.windows[winAgent] {
		t.Fatalf("expected repo-agent running")
	}
}

func TestStartSameProcessIsIdempotent(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api 1: %v", err)
	}
	// Start api again — should not create a second pane.
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api 2: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	panes := backend.panes[window]
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane (idempotent), got %d", len(panes))
	}
}

func TestRestartStopsAndRestartsProcess(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	// Start api and web.
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	if len(backend.panes[window]) != 2 {
		t.Fatalf("expected 2 panes before restart, got %d", len(backend.panes[window]))
	}

	// Restart api — should stop old api pane and create new one.
	if err := manager.Restart(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("restart api: %v", err)
	}

	panes := backend.panes[window]
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes after restart, got %d", len(panes))
	}
	// web should still be there, and api should be re-created.
	names := map[string]bool{}
	for _, p := range panes {
		names[tmux.ProcessFromPaneTitle(p.Title)] = true
	}
	if !names["api"] {
		t.Fatalf("expected api pane after restart")
	}
	if !names["web"] {
		t.Fatalf("expected web pane preserved after restart")
	}
}

func TestRestartGroupStopsAndRestartsEachProcess(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start group: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	if len(backend.panes[window]) != 2 {
		t.Fatalf("expected 2 panes before group restart, got %d", len(backend.panes[window]))
	}

	if err := manager.Restart(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("restart group: %v", err)
	}

	panes := backend.panes[window]
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes after group restart, got %d", len(panes))
	}
	names := map[string]bool{}
	for _, pane := range panes {
		names[tmux.ProcessFromPaneTitle(pane.Title)] = true
	}
	if !names["api"] || !names["web"] {
		t.Fatalf("expected both group members after restart, got %#v", names)
	}
}

func TestStopWorktreeKillsAllPanes(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}
	if err := manager.StopWorktree(ctx, "repo-main"); err != nil {
		t.Fatalf("stop worktree: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	if backend.windows[window] {
		t.Fatalf("expected window killed")
	}
	if len(backend.panes[window]) != 0 {
		t.Fatalf("expected all panes cleared")
	}
}

func TestStopAllKillsEverything(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start repo-main: %v", err)
	}
	if err := manager.Start(ctx, "repo-agent", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start repo-agent: %v", err)
	}
	if err := manager.StopAll(ctx); err != nil {
		t.Fatalf("stop all: %v", err)
	}

	for _, wt := range testWorktrees() {
		window := tmux.WindowName(wt.Dir)
		if backend.windows[window] {
			t.Fatalf("expected %q stopped after StopAll", wt.Name)
		}
	}
	if backend.options[tmux.ActiveWorktreeOptionKey()] != "" {
		t.Fatalf("expected active worktree cleared")
	}
}

func TestStopProcessErrorsWhenNotRunning(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	err := manager.StopProcess(ctx, "repo-main", "web")
	if err == nil {
		t.Fatalf("expected error stopping non-running process")
	}
}

func TestStopProcessClearsActiveWhenLastPaneStops(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("switch repo-main: %v", err)
	}
	if err := manager.StopProcess(ctx, "repo-main", "api"); err != nil {
		t.Fatalf("stop api: %v", err)
	}

	if backend.options[tmux.ActiveWorktreeOptionKey()] != "" {
		t.Fatalf("expected active worktree cleared, got %q", backend.options[tmux.ActiveWorktreeOptionKey()])
	}
	if backend.options[tmux.ActiveProcessOptionKey()] != "" {
		t.Fatalf("expected active process cleared, got %q", backend.options[tmux.ActiveProcessOptionKey()])
	}
}

func TestStopProcessRepointsActiveProcessToRemainingPane(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}
	if err := manager.StopProcess(ctx, "repo-main", "web"); err != nil {
		t.Fatalf("stop web: %v", err)
	}

	if backend.options[tmux.ActiveWorktreeOptionKey()] != "/tmp/repo-main" {
		t.Fatalf("expected active worktree preserved, got %q", backend.options[tmux.ActiveWorktreeOptionKey()])
	}
	if backend.options[tmux.ActiveProcessOptionKey()] != "api" {
		t.Fatalf("expected active process updated to api, got %q", backend.options[tmux.ActiveProcessOptionKey()])
	}
}

func TestStatusReportsMultipleProcesses(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("start web: %v", err)
	}

	rows, err := manager.Status(ctx, "repo-main")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for repo-main, got %d", len(rows))
	}
	row := rows[0]
	if !row.Running {
		t.Fatalf("expected running")
	}
	if len(row.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(row.Processes))
	}
	if row.Processes[0].Name != "api" || row.Processes[1].Name != "web" {
		t.Fatalf("unexpected process names: %v", row.Processes)
	}
	if row.Process != "api, web" {
		t.Fatalf("unexpected combined process string: %q", row.Process)
	}
}

func TestStatusStoppedWorktreeHasNoProcesses(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	rows, err := manager.Status(ctx, "repo-main")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Running {
		t.Fatalf("expected not running")
	}
	if len(rows[0].Processes) != 0 {
		t.Fatalf("expected no processes, got %d", len(rows[0].Processes))
	}
}

func TestLogsWithProcessTargetsSpecificPane(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}

	// Logs with specific process should not error.
	_, err := manager.Logs(ctx, "repo-main", "api", 100)
	if err != nil {
		t.Fatalf("logs with process: %v", err)
	}

	// Logs with non-running process should error.
	_, err = manager.Logs(ctx, "repo-main", "web", 100)
	if err == nil {
		t.Fatalf("expected error for non-running process logs")
	}
}

func TestLogsWithoutProcessFallsBackToWindow(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start api: %v", err)
	}

	// Logs without process should use window capture (not error).
	_, err := manager.Logs(ctx, "repo-main", "", 100)
	if err != nil {
		t.Fatalf("logs without process: %v", err)
	}
}

func TestLogsErrorsWhenWorktreeNotRunning(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	_, err := manager.Logs(ctx, "repo-main", "", 100)
	if err == nil {
		t.Fatalf("expected error for stopped worktree")
	}
}

func TestStopProcessMatchesLegacyPane(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	// Simulate a legacy worktree started without pane titles.
	window := tmux.WindowName("/tmp/repo-main")
	backend.windows[window] = true
	backend.panes[window] = []tmux.PaneInfo{
		{ID: "%0", Title: "fish", PID: "1234", Command: "node"}, // legacy: shell name as title, not wts: prefix
	}

	// StopProcess should match the legacy pane.
	if err := manager.StopProcess(ctx, "repo-main", "api"); err != nil {
		t.Fatalf("expected legacy pane to match, got: %v", err)
	}
	if backend.windows[window] {
		t.Fatalf("expected window stopped after killing sole legacy pane")
	}
}

func TestStatusDetectsExitedViaPanePID(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Simulate process exited: pane has returned to the shell and has no child process.
	window := tmux.WindowName("/tmp/repo-main")
	backend.panes[window][0].Command = "fish"
	backend.exitedByPID[backend.panes[window][0].PID] = true

	rows, err := manager.Status(ctx, "repo-main")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if !rows[0].Exited {
		t.Fatalf("expected exited=true when pane has no child process")
	}
	if !rows[0].Running {
		t.Fatalf("expected running=true (window still exists)")
	}
}

func TestStatusKeepsRunningWhenShellCommandButChildStillAlive(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Repro for the TUI bug: tmux reports the shell, but the pane still has a live child.
	window := tmux.WindowName("/tmp/repo-main")
	backend.panes[window][0].Command = "fish"

	rows, err := manager.Status(ctx, "repo-main")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if rows[0].Exited {
		t.Fatalf("expected exited=false while pane child process is still alive")
	}
}

func TestStatusUsesPaneProcessMetadataWhenTitleChanges(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start group: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-main")
	backend.panes[window][0].Title = "~/p/wps"
	backend.panes[window][1].Title = "/bin/sh -lc demo"

	rows, err := manager.Status(ctx, "repo-main")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(rows) != 1 || len(rows[0].Processes) != 2 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	names := map[string]bool{}
	for _, proc := range rows[0].Processes {
		names[proc.Name] = true
	}
	if !names["api"] || !names["web"] {
		t.Fatalf("expected pane metadata names preserved, got %#v", names)
	}
}

func TestSwitchPreemptsButStartDoesNot(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	// Start in repo-main.
	if err := manager.Start(ctx, "repo-main", RunOptions{Process: "api"}); err != nil {
		t.Fatalf("start repo-main: %v", err)
	}
	// Switch to repo-agent — should preempt repo-main.
	if err := manager.Switch(ctx, "repo-agent", RunOptions{Process: "web"}); err != nil {
		t.Fatalf("switch repo-agent: %v", err)
	}

	winMain := tmux.WindowName("/tmp/repo-main")
	winAgent := tmux.WindowName("/tmp/repo-agent")
	if backend.windows[winMain] {
		t.Fatalf("Switch should have stopped repo-main")
	}
	if !backend.windows[winAgent] {
		t.Fatalf("expected repo-agent running")
	}
}
