package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/state"
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
func (m *mockBackend) Attach(context.Context, string, string) error { return nil }

func testProject() *model.Project {
	cfg := model.Config{
		Version: model.CurrentVersion,
		Defaults: model.Defaults{
			StopTimeoutSec: model.DefaultStopTimeout,
			Shell:          model.DefaultShell,
		},
		Processes: []model.Process{
			{Name: "api", Command: "go run .", Group: "backend", Env: map[string]string{}},
			{Name: "worker", Command: "go run .", Group: "backend", Env: map[string]string{}},
			{Name: "web", Command: "pnpm dev", Group: "frontend", Env: map[string]string{}},
		},
	}
	return model.NewProject("/tmp/.wts.yaml", "/tmp", cfg)
}

func testRepo() *state.RepoState {
	return &state.RepoState{
		Root: "/tmp/repo",
		Worktrees: []state.Worktree{
			{Name: "api-main", Dir: "/tmp/api-main", Process: "api"},
			{Name: "api-agent", Dir: "/tmp/api-agent", Process: "worker"},
			{Name: "web-main", Dir: "/tmp/web-main", Process: "web"},
		},
	}
}

func TestSwitchStartsWorktreeAndMarksActive(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), testRepo(), backend)

	if err := manager.Switch(context.Background(), "api-main", SwitchOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	window := tmux.WindowName("api-main")
	if !backend.windows[window] {
		t.Fatalf("expected %q to be running", window)
	}
	key := tmux.GroupOptionKey("backend")
	if backend.options[key] != "api-main" {
		t.Fatalf("unexpected active backend value: %q", backend.options[key])
	}
}

func TestSwitchPreemptsSameGroup(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), testRepo(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api-main", SwitchOptions{}); err != nil {
		t.Fatalf("switch api-main: %v", err)
	}
	if err := manager.Switch(ctx, "api-agent", SwitchOptions{}); err != nil {
		t.Fatalf("switch api-agent: %v", err)
	}

	if backend.windows[tmux.WindowName("api-main")] {
		t.Fatalf("expected previous worktree stopped")
	}
	if !backend.windows[tmux.WindowName("api-agent")] {
		t.Fatalf("expected target worktree running")
	}
}

func TestSwitchDifferentGroupNoPreempt(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), testRepo(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api-main", SwitchOptions{}); err != nil {
		t.Fatalf("switch api-main: %v", err)
	}
	if err := manager.Switch(ctx, "web-main", SwitchOptions{}); err != nil {
		t.Fatalf("switch web-main: %v", err)
	}

	if !backend.windows[tmux.WindowName("api-main")] {
		t.Fatalf("expected api-main still running")
	}
	if !backend.windows[tmux.WindowName("web-main")] {
		t.Fatalf("expected web-main running")
	}
}
