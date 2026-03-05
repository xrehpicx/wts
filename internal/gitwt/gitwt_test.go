package gitwt

import (
	"path/filepath"
	"testing"
)

func TestParsePorcelain(t *testing.T) {
	t.Parallel()

	raw := `
worktree /tmp/repo
HEAD abcdef1234567890
branch refs/heads/main

worktree /tmp/repo-agent
HEAD 1234567890abcdef
branch refs/heads/agent

`

	items, err := parsePorcelain([]byte(raw))
	if err != nil {
		t.Fatalf("parsePorcelain: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(items))
	}
	if items[0].Name != "repo" {
		t.Fatalf("unexpected name: %q", items[0].Name)
	}
	if items[0].Branch != "main" {
		t.Fatalf("unexpected branch: %q", items[0].Branch)
	}
	if items[1].Name != "repo-agent" {
		t.Fatalf("unexpected name: %q", items[1].Name)
	}
}

func TestResolveByNameAndDir(t *testing.T) {
	t.Parallel()

	items := []Worktree{
		{Name: "repo", Dir: filepath.Clean("/tmp/repo")},
		{Name: "repo-agent", Dir: filepath.Clean("/tmp/repo-agent")},
	}

	byName, err := Resolve(items, "repo-agent")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if byName.Dir != filepath.Clean("/tmp/repo-agent") {
		t.Fatalf("unexpected dir from name resolve: %q", byName.Dir)
	}

	byDir, err := Resolve(items, "/tmp/repo")
	if err != nil {
		t.Fatalf("resolve by dir: %v", err)
	}
	if byDir.Name != "repo" {
		t.Fatalf("unexpected name from dir resolve: %q", byDir.Name)
	}
}
