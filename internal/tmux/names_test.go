package tmux

import (
	"strings"
	"testing"
)

func TestSessionNameIsDeterministic(t *testing.T) {
	t.Parallel()

	name1 := SessionName("/Users/raj/projects/wps")
	name2 := SessionName("/Users/raj/projects/wps")
	if name1 != name2 {
		t.Fatalf("expected deterministic session name")
	}
	if !strings.HasPrefix(name1, "wts_") {
		t.Fatalf("unexpected session prefix: %q", name1)
	}
	if strings.Contains(name1, ":") {
		t.Fatalf("session name should be tmux target-safe: %q", name1)
	}
}

func TestWindowName(t *testing.T) {
	t.Parallel()

	window := WindowName("/tmp/repo-api")
	if !strings.HasPrefix(window, "ws_") {
		t.Fatalf("unexpected window name prefix: %q", window)
	}
	if !strings.Contains(window, "_repo-api") {
		t.Fatalf("unexpected window name suffix: %q", window)
	}
}

func TestProcessPaneTitle(t *testing.T) {
	t.Parallel()

	title := ProcessPaneTitle("api")
	if title != "wts:api" {
		t.Fatalf("unexpected pane title: %q", title)
	}

	title = ProcessPaneTitle("auth:generate")
	if title != "wts:auth:generate" {
		t.Fatalf("unexpected pane title for colon name: %q", title)
	}
}

func TestIsShellCommand(t *testing.T) {
	t.Parallel()

	shells := []string{"fish", "bash", "zsh", "sh", "dash", "ksh"}
	for _, s := range shells {
		if !IsShellCommand(s) {
			t.Errorf("expected %q to be shell", s)
		}
	}

	nonShells := []string{"node", "go", "python", "make", "npm", ""}
	for _, s := range nonShells {
		if IsShellCommand(s) {
			t.Errorf("expected %q to NOT be shell", s)
		}
	}
}

func TestProcessFromPaneTitle(t *testing.T) {
	t.Parallel()

	if name := ProcessFromPaneTitle("wts:api"); name != "api" {
		t.Fatalf("expected api, got %q", name)
	}
	if name := ProcessFromPaneTitle("wts:auth:generate"); name != "auth:generate" {
		t.Fatalf("expected auth:generate, got %q", name)
	}
	if name := ProcessFromPaneTitle("bash"); name != "" {
		t.Fatalf("expected empty for non-wts title, got %q", name)
	}
	if name := ProcessFromPaneTitle(""); name != "" {
		t.Fatalf("expected empty for empty title, got %q", name)
	}
}
