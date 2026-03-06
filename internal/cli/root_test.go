package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveRepoRootUsesProvidedDirectory(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	cmd := exec.Command("git", "init", repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, string(out))
	}

	nestedDir := filepath.Join(repoDir, "subdir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	got, err := resolveRepoRoot(nestedDir)
	if err != nil {
		t.Fatalf("resolveRepoRoot: %v", err)
	}
	want, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", repoDir, err)
	}
	if got != want {
		t.Fatalf("resolveRepoRoot(%q) = %q; want %q", nestedDir, got, want)
	}
}
