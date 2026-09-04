package gitwt

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	agentDir := filepath.Join(root, "repo-agent")
	items := []Worktree{
		{Name: "repo", Dir: repoDir},
		{Name: "repo-agent", Dir: agentDir},
	}

	byName, err := Resolve(items, "repo-agent")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if byName.Dir != agentDir {
		t.Fatalf("unexpected dir from name resolve: %q", byName.Dir)
	}

	byDir, err := Resolve(items, repoDir)
	if err != nil {
		t.Fatalf("resolve by dir: %v", err)
	}
	if byDir.Name != "repo" {
		t.Fatalf("unexpected name from dir resolve: %q", byDir.Name)
	}
}

func TestParsePorcelainNULPreservesUnusualPathCharacters(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "worktree with space\nand newline")
	raw := "worktree " + dir + "\x00HEAD abcdef\x00branch refs/heads/feature\x00\x00"
	items, err := parsePorcelain([]byte(raw))
	if err != nil {
		t.Fatalf("parsePorcelain: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(items))
	}
	if items[0].Dir != filepath.Clean(dir) {
		t.Fatalf("path changed during parse: got %q; want %q", items[0].Dir, filepath.Clean(dir))
	}
}

func TestParsePorcelainNULPreservesTrailingCarriageReturn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "worktree\r")
	items, err := parsePorcelain([]byte("worktree " + dir + "\x00HEAD abc\x00prunable\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Dir != dir || !items[0].Prunable {
		t.Fatalf("unexpected parsed worktree: %#v", items)
	}
}

func TestResolvePreservesWhitespaceInPaths(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "tree ")
	items := []Worktree{{Name: "tree ", Dir: dir}}
	for _, selector := range []string{dir, "tree "} {
		if _, err := Resolve(items, selector); err != nil {
			t.Fatalf("resolve %q: %v", selector, err)
		}
	}
}

func TestDiscoverContextWithRealWorktrees(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=WTS Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("commit", "--allow-empty", "-m", "initial")
	worktree := filepath.Join(dir, "feature with space\nand newline\r")
	run("worktree", "add", "-b", "feature", worktree)
	items, err := DiscoverContext(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	// Git resolves macOS /var and /tmp symlinks in its porcelain output.
	canonical, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := Resolve(items, canonical)
	if err != nil || wt.Branch != "feature" {
		t.Fatalf("discover unusual path: %#v, %v", wt, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DiscoverContext(ctx, repo); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled discovery = %v", err)
	}
}
