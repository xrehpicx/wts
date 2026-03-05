package runtime

import (
	"context"
	"testing"
	"time"

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

func (m *mockBackend) EnsureTmux(context.Context) error            { return nil }
func (m *mockBackend) EnsureSession(context.Context, string) error { return nil }
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
		Workspaces: []model.Workspace{
			{Name: "api", ResolvedDir: "/tmp/api", Command: "go run .", EffectiveGroup: "backend", Env: map[string]string{}},
			{Name: "worker", ResolvedDir: "/tmp/worker", Command: "go run .", EffectiveGroup: "backend", Env: map[string]string{}},
			{Name: "web", ResolvedDir: "/tmp/web", Command: "pnpm dev", EffectiveGroup: "frontend", Env: map[string]string{}},
		},
	}
	return model.NewProject("/tmp/.wts.yaml", "/tmp", cfg)
}

func TestSwitchStartsWorkspaceAndMarksActive(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)

	if err := manager.Switch(context.Background(), "api", SwitchOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	window := tmux.WindowName("api")
	if !backend.windows[window] {
		t.Fatalf("expected %q to be running", window)
	}
	key := tmux.GroupOptionKey("backend")
	if backend.options[key] != "api" {
		t.Fatalf("unexpected active backend value: %q", backend.options[key])
	}
}

func TestSwitchPreemptsPreviousWorkspaceInSameGroup(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("switch api: %v", err)
	}
	if err := manager.Switch(ctx, "worker", SwitchOptions{}); err != nil {
		t.Fatalf("switch worker: %v", err)
	}

	apiWindow := tmux.WindowName("api")
	workerWindow := tmux.WindowName("worker")
	if backend.windows[apiWindow] {
		t.Fatalf("expected %q to be stopped", apiWindow)
	}
	if !backend.windows[workerWindow] {
		t.Fatalf("expected %q to be running", workerWindow)
	}
	if backend.stopCount[apiWindow] == 0 {
		t.Fatalf("expected %q to be preempted", apiWindow)
	}
}

func TestSwitchAcrossGroupsKeepsOtherGroupRunning(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("switch api: %v", err)
	}
	if err := manager.Switch(ctx, "web", SwitchOptions{}); err != nil {
		t.Fatalf("switch web: %v", err)
	}

	if !backend.windows[tmux.WindowName("api")] {
		t.Fatalf("expected api workspace to stay running")
	}
	if !backend.windows[tmux.WindowName("web")] {
		t.Fatalf("expected web workspace to be running")
	}
}

func TestStartPreemptsWithinGroup(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)
	ctx := context.Background()

	if err := manager.Start(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("start api: %v", err)
	}
	if err := manager.Start(ctx, "worker", SwitchOptions{}); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	if backend.stopCount[tmux.WindowName("api")] == 0 {
		t.Fatalf("expected api workspace to be preempted by start")
	}
}

func TestRestartRecreatesWorkspaceProcess(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("switch api: %v", err)
	}
	if err := manager.Restart(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("restart api: %v", err)
	}

	window := tmux.WindowName("api")
	if backend.stopCount[window] == 0 {
		t.Fatalf("expected stop call during restart")
	}
	if backend.startCount[window] < 2 {
		t.Fatalf("expected start to run twice, got %d", backend.startCount[window])
	}
}

func TestStopGroupStopsOnlyGroupActiveWorkspace(t *testing.T) {
	t.Parallel()

	backend := newMockBackend()
	manager := NewManager(testProject(), backend)
	ctx := context.Background()

	if err := manager.Switch(ctx, "api", SwitchOptions{}); err != nil {
		t.Fatalf("switch api: %v", err)
	}
	if err := manager.Switch(ctx, "web", SwitchOptions{}); err != nil {
		t.Fatalf("switch web: %v", err)
	}
	if err := manager.StopGroup(ctx, "backend"); err != nil {
		t.Fatalf("stop group backend: %v", err)
	}

	if backend.windows[tmux.WindowName("api")] {
		t.Fatalf("expected backend active workspace to be stopped")
	}
	if !backend.windows[tmux.WindowName("web")] {
		t.Fatalf("expected frontend workspace to remain running")
	}
}
