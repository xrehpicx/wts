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
	windows    map[string]bool
	options    map[string]string
	startCount map[string]int
	stopCount  map[string]int
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		windows:    map[string]bool{},
		options:    map[string]string{},
		startCount: map[string]int{},
		stopCount:  map[string]int{},
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
	if backend.options[tmux.ProcessOptionKey("/tmp/repo-agent")] != "web" {
		t.Fatalf("unexpected process option for worktree")
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
}
