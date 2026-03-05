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
