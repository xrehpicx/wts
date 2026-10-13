package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xrehpicx/wts/internal/gitwt"
	"github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

func TestValidateRunSelection(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		all    bool
		attach bool
		args   []string
		want   string
	}{
		{name: "one worktree", args: []string{"a"}},
		{name: "several worktrees", args: []string{"a", "b"}},
		{name: "all", all: true},
		{name: "attach one", attach: true, args: []string{"a"}},
		{name: "nothing selected", want: "at least one worktree"},
		{name: "all with selectors", all: true, args: []string{"a"}, want: "--all cannot be combined"},
		{name: "attach several", attach: true, args: []string{"a", "b"}, want: "--attach requires a single worktree"},
		{name: "attach all", attach: true, all: true, want: "--attach requires a single worktree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateRunSelection(tc.all, tc.attach, tc.args)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateStopSelectionAcceptsSeveralWorktrees(t *testing.T) {
	t.Parallel()
	if err := validateStopSelection(false, "", "dev", []string{"a", "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOnlyAdditiveCommandsAcceptSeveralWorktrees(t *testing.T) {
	t.Parallel()
	a := &app{}
	if err := a.newSwitchCmd().Args(nil, []string{"a", "b"}); err == nil {
		t.Fatal("switch preempts and must accept exactly one worktree")
	}
	for _, cmd := range []func() error{
		func() error { return a.newStartCmd().Args(nil, []string{"a", "b"}) },
		func() error { return a.newRestartCmd().Args(nil, []string{"a", "b"}) },
	} {
		if err := cmd(); err != nil {
			t.Fatalf("additive command rejected several worktrees: %v", err)
		}
	}
	if a.newStartCmd().Flags().Lookup("all") == nil {
		t.Fatal("start must offer --all")
	}
	if a.newRestartCmd().Flags().Lookup("all") != nil || a.newSwitchCmd().Flags().Lookup("all") != nil {
		t.Fatal("--all is only meaningful for start")
	}
}

// recordingBackend tracks started and stopped windows for TUI action tests.
type recordingBackend struct {
	*tuiTestBackend
	started map[string]int
	stopped map[string]int
}

func newRecordingBackend() *recordingBackend {
	return &recordingBackend{tuiTestBackend: newTUITestBackend(), started: map[string]int{}, stopped: map[string]int{}}
}

func (b *recordingBackend) StartWindowCommand(_ context.Context, _ string, window, _, _, _ string, _ map[string]string, paneTitle string) error {
	b.started[window]++
	b.windows[window] = true
	b.panes[window] = append(b.panes[window], tmux.PaneInfo{ID: "%" + window + paneTitle, Process: tmux.ProcessFromPaneTitle(paneTitle), Title: paneTitle, PID: "1", Command: "node"})
	return nil
}

func (b *recordingBackend) SplitWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string, paneTitle string) error {
	return b.StartWindowCommand(ctx, session, window, dir, shell, command, env, paneTitle)
}

func (b *recordingBackend) StopWindow(_ context.Context, _ string, window string, _ time.Duration) error {
	b.stopped[window]++
	b.windows[window] = false
	delete(b.panes, window)
	return nil
}

func markedTUI(t *testing.T) (*tuiModel, *recordingBackend) {
	t.Helper()
	project := testTUIProject()
	worktrees := []gitwt.Worktree{
		{Name: "one", Dir: "/tmp/one", Branch: "one"},
		{Name: "two", Dir: "/tmp/two", Branch: "two"},
		{Name: "three", Dir: "/tmp/three", Branch: "three"},
	}
	backend := newRecordingBackend()
	m := newTUIModel(&runtimeContext{
		ctx:       context.Background(),
		project:   project,
		repoRoot:  t.TempDir(),
		worktrees: worktrees,
		manager:   runtime.NewManager(project, "/tmp/repo", worktrees, backend),
	})
	m.rows = []runtime.StatusRow{
		{Worktree: "one", Dir: "/tmp/one"},
		{Worktree: "three", Dir: "/tmp/three"},
		{Worktree: "two", Dir: "/tmp/two"},
	}
	m.idx = 0
	return m, backend
}

func pressKey(m *tuiModel, k tea.KeyPressMsg) tea.Cmd {
	_, cmd := m.Update(k)
	return cmd
}

func TestTUISpaceMarksWorktreesAndAdvances(t *testing.T) {
	m, _ := markedTUI(t)
	space := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}

	pressKey(m, space)
	if !m.marked["/tmp/one"] || m.idx != 1 {
		t.Fatalf("space should mark and advance: marked=%v idx=%d", m.marked, m.idx)
	}
	m.idx = 2
	pressKey(m, space)
	if !m.marked["/tmp/two"] || m.idx != 2 {
		t.Fatalf("marking the last row must not wrap: marked=%v idx=%d", m.marked, m.idx)
	}
	pressKey(m, space)
	if m.marked["/tmp/two"] {
		t.Fatal("space should toggle an existing mark off")
	}

	view := m.View().Content
	if !strings.Contains(view, "1 worktree marked") || !strings.Contains(view, "✓") {
		t.Fatalf("expected marked worktree indicators in view:\n%s", view)
	}

	pressKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.marked) != 0 {
		t.Fatalf("esc should clear marks, got %v", m.marked)
	}
}

func TestTUIStartsSelectedTargetInEveryMarkedWorktree(t *testing.T) {
	m, backend := markedTUI(t)
	backend.options[tmux.ActiveWorktreeOptionKey()] = "/tmp/three"
	m.marked = map[string]bool{"/tmp/one": true, "/tmp/two": true}
	m.selectTarget(m.rc.project.Targets()[2]) // [group] dev

	msgs := runCmdMessages(pressKey(m, tea.KeyPressMsg{Code: tea.KeyEnter}))
	var done *actionDoneMsg
	for _, msg := range msgs {
		if d, ok := msg.(actionDoneMsg); ok {
			done = &d
		}
		if e, ok := msg.(actionErrMsg); ok {
			t.Fatalf("action failed: %v", e.err)
		}
	}
	if done == nil || done.text != "started [group] dev in 2 worktrees" {
		t.Fatalf("unexpected result: %+v", msgs)
	}
	for _, dir := range []string{"/tmp/one", "/tmp/two"} {
		if got := backend.started[tmux.WindowName(dir)]; got != 2 {
			t.Fatalf("expected both group members in %s, started %d", dir, got)
		}
	}
	if backend.started[tmux.WindowName("/tmp/three")] != 0 || backend.stopped[tmux.WindowName("/tmp/three")] != 0 {
		t.Fatal("marked starts must not touch or preempt unmarked worktrees")
	}
	if len(m.marked) != 2 {
		t.Fatal("marks should persist so the same worktrees can be restarted or stopped")
	}
}

func TestTUIStopTargetSkipsMarkedWorktreesWhereItIsNotRunning(t *testing.T) {
	m, _ := markedTUI(t)
	m.marked = map[string]bool{"/tmp/one": true, "/tmp/two": true}
	m.selectTarget(m.rc.project.Targets()[0]) // api

	if cmd := m.stopCurrentCmd(); cmd != nil || !m.messageIsErr || !strings.Contains(m.message, "any marked worktree") {
		t.Fatalf("expected a not-running error, got cmd=%v message=%q", cmd != nil, m.message)
	}

	m.rows[0].Processes = []runtime.ProcessStatus{{Name: "api", Running: true}}
	m.message, m.messageIsErr = "", false
	if cmd := m.stopCurrentCmd(); cmd == nil {
		t.Fatalf("expected stop to run in the worktree where api runs: %q", m.message)
	}
	if !strings.Contains(m.loadingMsg, "in 1 worktree") || !m.loadingDirs["/tmp/one"] || m.loadingDirs["/tmp/two"] {
		t.Fatalf("stop should only include running worktrees: %q %v", m.loadingMsg, m.loadingDirs)
	}
}

func TestTUIRefreshForgetsMarksForRemovedWorktrees(t *testing.T) {
	m, _ := markedTUI(t)
	m.marked = map[string]bool{"/tmp/one": true, "/tmp/two": true}
	m.statusRequest = 7
	m.Update(statusRefreshedMsg{request: 7, rows: []runtime.StatusRow{
		{Worktree: "one", Dir: "/tmp/one", Prunable: true},
		{Worktree: "two", Dir: "/tmp/two"},
	}})
	if m.marked["/tmp/one"] || !m.marked["/tmp/two"] || len(m.marked) != 1 {
		t.Fatalf("unexpected marks after refresh: %v", m.marked)
	}
}
