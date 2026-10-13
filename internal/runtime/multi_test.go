package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/tmux"
)

func startEach(ctx context.Context, manager *Manager, selectors []string, opts RunOptions) error {
	return manager.ForEachWorktree(ctx, selectors, func(ctx context.Context, wt gitwt.Worktree) error {
		return manager.Start(ctx, wt.Dir, opts)
	})
}

func TestForEachWorktreeRunsSameGroupInEveryWorktree(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()

	if err := startEach(ctx, manager, []string{"repo-main", "repo-agent"}, RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start group in both worktrees: %v", err)
	}

	rows, err := manager.Status(ctx, "")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, row := range rows {
		if !row.Running || len(row.Processes) != 2 {
			t.Fatalf("expected both group members in %s, got %+v", row.Worktree, row.Processes)
		}
	}
	for _, dir := range []string{"/tmp/repo-main", "/tmp/repo-agent"} {
		if backend.stopCount[tmux.WindowName(dir)] != 0 {
			t.Fatalf("starting in several worktrees must not preempt %s", dir)
		}
	}
}

func TestForEachWorktreeResolvesEverySelectorBeforeRunning(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)

	err := startEach(context.Background(), manager, []string{"repo-main", "missing"}, RunOptions{})
	if err == nil || !strings.Contains(err.Error(), `"missing" not found`) {
		t.Fatalf("expected unknown worktree error, got %v", err)
	}
	if backend.windows[tmux.WindowName("/tmp/repo-main")] {
		t.Fatal("no worktree should start when any selector is invalid")
	}
}

func TestForEachWorktreeVisitsDuplicateSelectorsOnce(t *testing.T) {
	t.Parallel()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), newMockBackend())
	var visited []string
	err := manager.ForEachWorktree(context.Background(), []string{"repo-main", "/tmp/repo-main", "repo-agent"}, func(_ context.Context, wt gitwt.Worktree) error {
		visited = append(visited, wt.Name)
		return nil
	})
	if err != nil {
		t.Fatalf("for each: %v", err)
	}
	if strings.Join(visited, ",") != "repo-main,repo-agent" {
		t.Fatalf("visited %v", visited)
	}
}

func TestForEachWorktreeContinuesAndLabelsFailures(t *testing.T) {
	t.Parallel()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), newMockBackend())
	failure := errors.New("boom")
	var visited int
	err := manager.ForEachWorktree(context.Background(), []string{"repo-main", "repo-agent"}, func(_ context.Context, wt gitwt.Worktree) error {
		visited++
		if wt.Name == "repo-main" {
			return failure
		}
		return nil
	})
	if visited != 2 {
		t.Fatalf("expected every worktree to be attempted, visited %d", visited)
	}
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "repo-main: boom") {
		t.Fatalf("expected labelled failure, got %v", err)
	}
}

func TestForEachWorktreeStopsWhenCanceled(t *testing.T) {
	t.Parallel()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), newMockBackend())
	ctx, cancel := context.WithCancel(context.Background())
	var visited int
	err := manager.ForEachWorktree(ctx, []string{"repo-main", "repo-agent"}, func(context.Context, gitwt.Worktree) error {
		visited++
		cancel()
		return nil
	})
	if visited != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("visited %d, err %v", visited, err)
	}
}

func TestForEachWorktreeRequiresSelector(t *testing.T) {
	t.Parallel()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), newMockBackend())
	if err := manager.ForEachWorktree(context.Background(), nil, nil); err == nil {
		t.Fatal("expected an error without selectors")
	}
}

func TestAvailableWorktreesOmitsUnusableEntries(t *testing.T) {
	t.Parallel()
	worktrees := append(testWorktrees(),
		gitwt.Worktree{Name: "bare", Dir: "/tmp/bare", Bare: true},
		gitwt.Worktree{Name: "gone", Dir: "/tmp/gone", Prunable: true},
	)
	manager := NewManager(testProject(), "/tmp/repo-main", worktrees, newMockBackend())
	var names []string
	for _, wt := range manager.AvailableWorktrees() {
		names = append(names, wt.Name)
	}
	if strings.Join(names, ",") != "repo-agent,repo-main" {
		t.Fatalf("available worktrees = %v", names)
	}
	if len(manager.ListWorktrees()) != 4 {
		t.Fatal("filtering must not modify the inventory")
	}
}

func TestProcessesReceiveWorktreeEnvironment(t *testing.T) {
	t.Parallel()
	project := model.NewProject("/tmp/.wts.yaml", "/tmp", model.Config{
		Version:  model.CurrentVersion,
		Defaults: model.Defaults{StopTimeoutSec: 1, Shell: model.DefaultShell},
		Processes: []model.Process{
			{Name: "api", Command: "serve", Env: map[string]string{"PORT": "4000", "WTS_BRANCH": "custom"}},
			{Name: "web", Command: "pnpm dev"},
		},
		Groups: []model.ProcessGroup{{Name: "dev", Processes: []string{"api", "web"}}},
	})
	backend := newMockBackend()
	manager := NewManager(project, "/tmp/repo-main", testWorktrees(), backend)
	if err := manager.Start(context.Background(), "repo-agent", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	window := tmux.WindowName("/tmp/repo-agent")
	web := backend.envByPane[window+"/"+tmux.ProcessPaneTitle("web")]
	want := map[string]string{
		"WTS_WORKTREE":     "repo-agent",
		"WTS_WORKTREE_DIR": "/tmp/repo-agent",
		"WTS_BRANCH":       "agent",
		"WTS_PROCESS":      "web",
	}
	for key, value := range want {
		if web[key] != value {
			t.Fatalf("%s = %q; want %q (env %v)", key, web[key], value, web)
		}
	}
	api := backend.envByPane[window+"/"+tmux.ProcessPaneTitle("api")]
	if api["PORT"] != "4000" || api["WTS_BRANCH"] != "custom" {
		t.Fatalf("configured env must be kept and take precedence: %v", api)
	}
}

func TestPartialSwitchRecordsWhereProcessesRun(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()
	if err := manager.Switch(ctx, "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	backend.splitErr = errors.New("no space for new pane")
	if err := manager.Switch(ctx, "repo-agent", RunOptions{Group: "dev"}); err == nil {
		t.Fatal("expected the second group member to fail")
	}
	if got := backend.options[tmux.ActiveWorktreeOptionKey()]; got != "/tmp/repo-agent" {
		t.Fatalf("active worktree = %q; the started member runs in repo-agent", got)
	}

	// A later switch must therefore stop what runs in repo-agent.
	backend.splitErr = nil
	if err := manager.Switch(ctx, "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if backend.windows[tmux.WindowName("/tmp/repo-agent")] {
		t.Fatal("processes from the partial switch were left running")
	}
}

func TestFailedSwitchClearsMarkersForStoppedWorktree(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()
	if err := manager.Switch(ctx, "repo-main", RunOptions{}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	backend.startErr = errors.New("tmux failed")
	if err := manager.Switch(ctx, "repo-agent", RunOptions{}); err == nil {
		t.Fatal("expected switch to fail")
	}
	if got := backend.options[tmux.ActiveWorktreeOptionKey()]; got != "" {
		t.Fatalf("active worktree = %q; nothing runs after the failed switch", got)
	}
}

func TestStopAllStopsWindowsOfRemovedWorktrees(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	orphan := tmux.WindowName("/tmp/removed")
	backend.windows[orphan] = true
	backend.windows["user-shell"] = true

	if err := manager.StopAll(context.Background()); err != nil {
		t.Fatalf("stop all: %v", err)
	}
	if backend.windows[orphan] {
		t.Fatal("window of a removed worktree is still running")
	}
	if !backend.windows["user-shell"] {
		t.Fatal("windows not created by wts must be left alone")
	}
}

func TestLogsWithoutProcessReadsFirstPane(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	ctx := context.Background()
	if err := manager.Start(ctx, "repo-main", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	first := backend.panes[tmux.WindowName("/tmp/repo-main")][0].ID
	if _, err := manager.Logs(ctx, "repo-main", "", 10); err != nil {
		t.Fatalf("logs: %v", err)
	}
	if backend.capturedPane != first {
		t.Fatalf("captured %q; want first pane %q", backend.capturedPane, first)
	}
}
