package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/xrehpicx/wts/internal/model"
)

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
