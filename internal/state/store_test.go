package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsEmptyState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.yaml"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	f, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Version != CurrentVersion {
		t.Fatalf("unexpected version: %d", f.Version)
	}
	if len(f.Repos) != 0 {
		t.Fatalf("expected no repos")
	}
}

func TestEnsureRepoAndSaveRoundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	store, err := NewStore(statePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	f := &File{Version: CurrentVersion}
	repo, _, err := f.EnsureRepo(dir)
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	repo.Worktrees = append(repo.Worktrees, Worktree{
		Name:    "main",
		Dir:     dir,
		Process: "dev",
	})

	if err := store.Save(f); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file missing: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load roundtrip: %v", err)
	}
	if len(loaded.Repos) != 1 || len(loaded.Repos[0].Worktrees) != 1 {
		t.Fatalf("unexpected loaded shape: %+v", loaded)
	}
}
