package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xrehpicx/wts/internal/model"
)

func TestLoadAssignsDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: dev
    command: "go run ./cmd/api"
    group: backend
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	project, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if project.Defaults.StopTimeoutSec != model.DefaultStopTimeout {
		t.Fatalf("unexpected stop timeout: got %d", project.Defaults.StopTimeoutSec)
	}
	if project.Defaults.Shell != model.DefaultShell {
		t.Fatalf("unexpected default shell: got %q", project.Defaults.Shell)
	}
	if len(project.Processes) != 1 {
		t.Fatalf("unexpected process count: %d", len(project.Processes))
	}
}

func TestLoadRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 2
processes:
  - name: dev
    command: "go run ."
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestLoadRejectsLegacyWorkspaceSchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
workspaces:
  - name: old
    dir: .
    command: "echo old"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for legacy workspaces schema")
	}
}
