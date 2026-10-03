package cli

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xrehpicx/wts/internal/tmux"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=wts", "-c", "user.email=wts@example.invalid"}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// Every worktree of a repository must map to the same tmux session; otherwise
// wts run from one worktree cannot see processes started from another.
func TestResolveSessionRootIsSharedByAllWorktrees(t *testing.T) {
	t.Parallel()
	temp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mainDir := filepath.Join(temp, "imai")
	featureDir := filepath.Join(temp, "t3code-seed-e2e-members-badge")
	gitIn(t, temp, "init", "-q", mainDir)
	gitIn(t, mainDir, "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, mainDir, "worktree", "add", "-q", "--detach", featureDir)

	ctx := context.Background()
	for _, dir := range []string{mainDir, featureDir, filepath.Join(featureDir, ".")} {
		repoRoot, err := resolveRepoRoot(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := resolveSessionRoot(ctx, dir, repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		if got != mainDir {
			t.Fatalf("resolveSessionRoot(%q) = %q; want main worktree %q", dir, got, mainDir)
		}
	}
	// The main worktree keeps the session name wts 0.4.0 gave it.
	mainRepoRoot, err := resolveRepoRoot(ctx, mainDir)
	if err != nil {
		t.Fatal(err)
	}
	if tmux.SessionName(mainRepoRoot) != tmux.SessionName(mainDir) {
		t.Fatal("main worktree session name changed")
	}
}

func TestResolveSessionRootUsesBareRepositoryDir(t *testing.T) {
	t.Parallel()
	temp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(temp, "seed")
	bare := filepath.Join(temp, "repo.git")
	featureDir := filepath.Join(temp, "feature")
	gitIn(t, temp, "init", "-q", seed)
	gitIn(t, seed, "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, temp, "clone", "-q", "--bare", seed, bare)
	gitIn(t, bare, "worktree", "add", "-q", "--detach", featureDir)

	ctx := context.Background()
	repoRoot, err := resolveRepoRoot(ctx, featureDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveSessionRoot(ctx, featureDir, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != bare {
		t.Fatalf("resolveSessionRoot() = %q; want bare repository %q", got, bare)
	}
}
