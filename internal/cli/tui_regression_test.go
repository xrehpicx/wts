package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrehpicx/wts/internal/config"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/xrehpicx/wts/internal/model"
	"github.com/xrehpicx/wts/internal/runtime"
)

func regressionTUI(t *testing.T) *tuiModel {
	t.Helper()
	project := testTUIProject()
	m := newTUIModel(&runtimeContext{project: project, repoRoot: t.TempDir(), manager: runtime.NewManager(project, "/tmp/repo", nil, newTUITestBackend())})
	m.rows = []runtime.StatusRow{{Worktree: "selected-tree", Dir: "/tmp/selected", Branch: "feature/ui"}}
	return m
}

func TestTUIViewBoundsAcrossModes(t *testing.T) {
	for _, mode := range []string{"normal", "help", "group", "filter", "empty"} {
		t.Run(mode, func(t *testing.T) {
			m := regressionTUI(t)
			switch mode {
			case "help":
				m.showAll = true
			case "group":
				m.enterCreateGroupMode()
			case "filter":
				m.enterFilterMode()
			case "empty":
				m.rows = nil
			}
			for _, w := range []int{1, 4, 12, 24, 48, 80, 87, 88, 120} {
				for _, h := range []int{1, 4, 5, 6, 8, 12, 20, 24, 40} {
					m.width, m.height = w, h
					got := m.View().Content
					if lipgloss.Width(got) > w || lipgloss.Height(got) > h {
						t.Errorf("%dx%d rendered %dx%d", w, h, lipgloss.Width(got), lipgloss.Height(got))
					}
				}
			}
		})
	}
}

func TestTUIFeedbackSurvivesNarrowWidth(t *testing.T) {
	m := regressionTUI(t)
	m.message, m.messageIsErr = "group name is required", true
	got := m.renderHeader(32)
	if !strings.Contains(got, "group name is required") {
		t.Fatalf("error hidden: %s", got)
	}
}

func TestTUIHeaderDoesNotAssociateSelectedTargetWithOtherActiveTree(t *testing.T) {
	m := regressionTUI(t)
	m.rows = append(m.rows, runtime.StatusRow{Worktree: "other-tree", Active: true, Running: true})
	got := m.renderHeader(100)
	if !strings.Contains(got, "selected-tree") {
		t.Fatalf("header omits action destination: %s", got)
	}
}

func TestTUIFilterCanNavigateAllMatches(t *testing.T) {
	m := regressionTUI(t)
	m.targets = []model.Target{{Name: "api-one"}, {Name: "web"}, {Name: "api-two"}}
	m.enterFilterMode()
	m.filterInput.SetValue("api")
	m.filterProcesses("api")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.targetIdx != 2 {
		t.Fatalf("down selected %d; want second match", m.targetIdx)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.targetIdx != 0 {
		t.Fatalf("up selected %d; want first match", m.targetIdx)
	}
}

func TestTUICtrlCQuitsFromEditors(t *testing.T) {
	for _, mode := range []string{"group", "filter"} {
		t.Run(mode, func(t *testing.T) {
			m := regressionTUI(t)
			if mode == "group" {
				m.enterCreateGroupMode()
			} else {
				m.enterFilterMode()
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
			if cmd == nil {
				t.Fatal("ctrl+c ignored")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("ctrl+c did not quit")
			}
		})
	}
}

func TestTUIGroupSaveCannotBeSubmittedTwice(t *testing.T) {
	m := regressionTUI(t)
	m.enterCreateGroupMode()
	m.createGroupInput.SetValue("new-group")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.loading {
		t.Fatal("first save did not start")
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("second save started while already saving")
	}
}

func TestTUIGroupLogsUseSpareCapacity(t *testing.T) {
	m := regressionTUI(t)
	m.logLines = map[string][]string{"api": {"a1"}, "web": {"w1", "w2", "w3", "w4", "w5", "w6"}}
	target := model.Target{Kind: model.TargetGroup, ProcessNames: []string{"api", "web"}}
	got := m.renderGroupLogs(target, 6, 40)
	if len(got) != 6 {
		t.Fatalf("unused log capacity: got %d lines, want 6", len(got))
	}
	for _, line := range got {
		if lipgloss.Width(line) > 40 {
			t.Errorf("overwide log: %s", line)
		}
	}
}

func TestTUIGroupEditorShowsMembersAtNormalNarrowSize(t *testing.T) {
	m := regressionTUI(t)
	m.width, m.height = 80, 24
	m.enterCreateGroupMode()
	got := m.View().Content
	if !strings.Contains(got, "[x] api") || !strings.Contains(got, "[ ] web") {
		t.Fatalf("members hidden: %s", got)
	}
}

func TestTUIUnusualNamesStayOnOneLine(t *testing.T) {
	m := regressionTUI(t)
	m.rows[0].Worktree = "line\nbreak\tname"
	got := m.renderListPanel(40, 12)
	if strings.Contains(got, "line\nbreak") {
		t.Fatal("worktree name injected a new display row")
	}
	if n := lipgloss.Height(got); n != 12 {
		t.Fatalf("got %d lines", n)
	}
}

func TestTUIRejectsStaleRefreshes(t *testing.T) {
	m := regressionTUI(t)
	m.statusRequest, m.logRequest = 2, 2
	m.logDir = m.rows[0].Dir
	m.logLines = map[string][]string{"api": {"fresh"}}
	m.Update(statusRefreshedMsg{request: 1, rows: []runtime.StatusRow{{Worktree: "stale"}}})
	m.Update(logsMsg{request: 1, dir: m.logDir, linesByTarget: map[string][]string{"api": {"stale"}}})
	if m.rows[0].Worktree != "selected-tree" || m.logLines["api"][0] != "fresh" {
		t.Fatal("stale refresh overwrote current state")
	}
	m.Update(logsMsg{request: 2, dir: "other", linesByTarget: map[string][]string{"api": {"wrong worktree"}}})
	if m.logLines["api"][0] != "fresh" {
		t.Fatal("logs came from the wrong worktree")
	}
}

func TestTUIRefreshKeepsSelectedWorktreeAfterReordering(t *testing.T) {
	m := regressionTUI(t)
	row := m.rows[0]
	m.Update(statusRefreshedMsg{rows: []runtime.StatusRow{{Worktree: "new-tree"}, row}})
	if m.idx != 1 {
		t.Fatal("refresh changed selected worktree")
	}
}

func TestTUITargetNavigationFetchesLogsImmediately(t *testing.T) {
	m := regressionTUI(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if cmd == nil {
		t.Fatal("target navigation did not request logs")
	}
	if m.logRequest != 1 {
		t.Fatal("target navigation did not invalidate old logs")
	}
}

func TestTUIFilterNoMatchesRestoresPreviousTargetOnEnter(t *testing.T) {
	m := regressionTUI(t)
	m.enterFilterMode()
	m.filterProcesses("no match")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.targetIdx != m.preFilterIdx {
		t.Fatal("search cleared previous target")
	}
}

func TestTUICompactListShowsFourWorktrees(t *testing.T) {
	m := regressionTUI(t)
	m.width, m.height = 48, 20
	m.rows = nil
	for i := 0; i < 12; i++ {
		m.rows = append(m.rows, runtime.StatusRow{Worktree: fmt.Sprintf("tree-%02d", i)})
	}
	got := m.View().Content
	for i := 0; i < 4; i++ {
		if !strings.Contains(got, fmt.Sprintf("tree-%02d", i)) {
			t.Fatalf("worktree %d hidden: %s", i, got)
		}
	}
}

func TestTUIPeriodicRefreshDoesNotStarveSlowRequests(t *testing.T) {
	m := regressionTUI(t)
	m.fetchLogsCmd()
	m.refreshStatusCmd()
	logRequest, statusRequest := m.logRequest, m.statusRequest
	m.Update(tickLogsMsg{})
	if m.logRequest != logRequest || m.statusRequest != statusRequest {
		t.Fatal("timer invalidated an in-flight refresh")
	}
	m.Update(logsMsg{request: logRequest, dir: m.rows[0].Dir})
	m.Update(statusRefreshedMsg{request: statusRequest, rows: m.rows})
	m.Update(tickLogsMsg{})
	if m.logRequest == logRequest || m.statusRequest == statusRequest {
		t.Fatal("timer failed to refresh after completion")
	}
}

func TestTUILogCommandUsesCapturedManager(t *testing.T) {
	m := regressionTUI(t)
	cmd := m.fetchLogsCmd()
	// Replacing runtime state while a command is queued must not affect it.
	m.rc.manager = nil
	if _, ok := cmd().(logsMsg); !ok {
		t.Fatal("queued log request did not finish")
	}
}

func TestTUIGroupSaveRoundTrip(t *testing.T) {
	m := regressionTUI(t)
	path := filepath.Join(t.TempDir(), ".wts.yaml")
	project, err := config.Save(path, m.rc.project.Config())
	if err != nil {
		t.Fatal(err)
	}
	m.rc.project = project
	m.rc.newBackend = func() runtime.Backend { return newTUITestBackend() }
	m.enterCreateGroupMode()
	m.createGroupInput.SetValue("backend")
	m.createGroupSelected = map[string]bool{"api": true}
	msgs := runCmdMessages(m.saveCreateGroupCmd())
	saved := false
	for _, msg := range msgs {
		if _, ok := msg.(groupCreatedMsg); ok {
			m.Update(msg)
			saved = true
		}
	}
	if !saved || m.createGroupMode || m.loading {
		t.Fatal("save did not complete and close the editor")
	}
	target, ok := m.selectedTarget()
	if !ok || target.Name != "backend" || target.Kind != model.TargetGroup {
		t.Fatal("saved group was not selected")
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loaded.ResolveTarget("", "backend"); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Processes) != 2 || len(loaded.Groups) != 2 {
		t.Fatal("saving a group lost existing configuration")
	}
}

func TestTUIGroupValidationErrorKeepsEditorOpen(t *testing.T) {
	m := regressionTUI(t)
	m.enterCreateGroupMode()
	if cmd := m.saveCreateGroupCmd(); cmd != nil || !m.messageIsErr {
		t.Fatal("empty group name accepted")
	}
	m.createGroupInput.SetValue("backend")
	m.createGroupSelected = nil
	if cmd := m.saveCreateGroupCmd(); cmd != nil || !m.messageIsErr {
		t.Fatal("empty membership accepted")
	}
	m.loading = true
	m.Update(actionErrMsg{err: errors.New("config is read-only")})
	if m.loading || !m.createGroupMode || !m.messageIsErr || m.message != "config is read-only" {
		t.Fatal("failed save lost editor state or error")
	}
}
