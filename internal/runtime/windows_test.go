package runtime

import (
	"context"
	"testing"

	"github.com/xrehpicx/wts/internal/tmux"
)

// legacySession reproduces wts 0.4.0 state: the repo-agent worktree's group
// was started from inside repo-agent, so its window lives in a session named
// after repo-agent instead of the manager's session.
const legacySession = "wts_legacy_repo-agent"

func newLegacyWindowBackend(t *testing.T) (*mockBackend, *Manager, string) {
	t.Helper()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	if manager.Session() == legacySession {
		t.Fatal("test needs the window outside the manager's session")
	}
	window := tmux.WindowName("/tmp/repo-agent")
	backend.windows[window] = true
	backend.windowSession[window] = legacySession
	backend.panes[window] = []tmux.PaneInfo{
		{ID: "%7", Process: "api", Title: "wts:api", PID: "2001", Command: "node"},
		{ID: "%8", Process: "web", Title: "wts:web", PID: "2002", Command: "node"},
	}
	backend.foreignOptions[legacySession] = map[string]string{
		tmux.ActiveWorktreeOptionKey():   "/tmp/repo-agent",
		tmux.ActiveProcessOptionKey():    "api",
		tmux.ActiveTargetKindOptionKey(): "group",
		tmux.ActiveTargetNameOptionKey(): "dev",
	}
	return backend, manager, window
}

func TestStopWorktreeStopsWindowOwnedByAnotherSession(t *testing.T) {
	t.Parallel()
	backend, manager, window := newLegacyWindowBackend(t)

	stopped, err := manager.StopWorktree(context.Background(), "/tmp/repo-agent")
	if err != nil {
		t.Fatalf("stop worktree: %v", err)
	}
	if !stopped {
		t.Fatal("expected the legacy window to count as running")
	}
	if backend.windows[window] || len(backend.panes[window]) != 0 {
		t.Fatalf("expected every process stopped; windows=%v panes=%v", backend.windows, backend.panes[window])
	}
	want := windowRef{Session: legacySession, Window: window}
	found := false
	for _, call := range backend.stopWindowCalls {
		if call == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("StopWindow calls %v; want one targeting %v", backend.stopWindowCalls, want)
	}
	if got := backend.foreignOptions[legacySession][tmux.ActiveWorktreeOptionKey()]; got != "" {
		t.Fatalf("legacy session still marks %q active", got)
	}
}

func TestStopWorktreeReportsWhenNothingIsRunning(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)

	stopped, err := manager.StopWorktree(context.Background(), "repo-agent")
	if err != nil {
		t.Fatalf("stop worktree: %v", err)
	}
	if stopped {
		t.Fatal("expected StopWorktree to report that nothing was running")
	}
}

func TestStopGroupFindsPanesInAnotherSession(t *testing.T) {
	t.Parallel()
	backend, manager, window := newLegacyWindowBackend(t)

	if err := manager.StopGroup(context.Background(), "/tmp/repo-agent", "dev"); err != nil {
		t.Fatalf("stop group: %v", err)
	}
	if backend.windows[window] {
		t.Fatal("expected group panes stopped")
	}
	if got := backend.foreignOptions[legacySession][tmux.ActiveWorktreeOptionKey()]; got != "" {
		t.Fatalf("legacy session still marks %q active", got)
	}
}

func TestStopProcessFindsPaneInAnotherSession(t *testing.T) {
	t.Parallel()
	backend, manager, window := newLegacyWindowBackend(t)

	if err := manager.StopProcess(context.Background(), "repo-agent", "web"); err != nil {
		t.Fatalf("stop process: %v", err)
	}
	panes := backend.panes[window]
	if len(panes) != 1 || panes[0].Process != "api" {
		t.Fatalf("expected only api to remain, got %#v", panes)
	}
}

func TestStatusReportsWindowOwnedByAnotherSession(t *testing.T) {
	t.Parallel()
	_, manager, _ := newLegacyWindowBackend(t)

	rows, err := manager.Status(context.Background(), "/tmp/repo-agent")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	row := rows[0]
	if !row.Running || !row.Active || len(row.Processes) != 2 {
		t.Fatalf("expected running active group, got %#v", row)
	}
	if row.Processes[0].Name != "api" || row.Processes[1].Name != "web" {
		t.Fatalf("unexpected processes: %#v", row.Processes)
	}
}

func TestLogsAndAttachUseOwningSession(t *testing.T) {
	t.Parallel()
	_, manager, window := newLegacyWindowBackend(t)
	ctx := context.Background()

	if _, err := manager.Logs(ctx, "repo-agent", "", 20); err != nil {
		t.Fatalf("logs: %v", err)
	}
	spec, err := manager.ResolveAttach(ctx, "repo-agent", RunOptions{Process: "web"})
	if err != nil {
		t.Fatalf("resolve attach: %v", err)
	}
	if spec.Session != legacySession || spec.Window != window || spec.PaneID != "%8" {
		t.Fatalf("attach spec %#v; want %s:%s pane %%8", spec, legacySession, window)
	}
}

func TestStartAddsMissingProcessToWindowInAnotherSession(t *testing.T) {
	t.Parallel()
	backend, manager, window := newLegacyWindowBackend(t)
	backend.panes[window] = backend.panes[window][:1]

	if err := manager.Start(context.Background(), "repo-agent", RunOptions{Group: "dev"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if backend.windowSession[window] != legacySession {
		t.Fatalf("window moved to %q; want it kept in %q", backend.windowSession[window], legacySession)
	}
	if got := len(backend.panes[window]); got != 2 {
		t.Fatalf("expected web split into the existing window, got %d panes", got)
	}
}

func TestLocateWindowsIgnoresSessionsWtsDoesNotManage(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	window := tmux.WindowName("/tmp/repo-agent")
	backend.windows[window] = true
	backend.windowSession[window] = "scratch"

	refs, err := manager.locateWindows(context.Background(), "/tmp/repo-agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected no managed windows, got %v", refs)
	}
}

func TestLocateWindowsPrefersManagerSession(t *testing.T) {
	t.Parallel()
	backend := newMockBackend()
	manager := NewManager(testProject(), "/tmp/repo-main", testWorktrees(), backend)
	if err := manager.Start(context.Background(), "repo-agent", RunOptions{Process: "api"}); err != nil {
		t.Fatal(err)
	}

	ref, exists, err := manager.locateWindow(context.Background(), "/tmp/repo-agent")
	if err != nil {
		t.Fatal(err)
	}
	want := windowRef{Session: manager.Session(), Window: tmux.WindowName("/tmp/repo-agent")}
	if !exists || ref != want {
		t.Fatalf("locateWindow = %v, %v; want %v", ref, exists, want)
	}
}
