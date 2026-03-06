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

func TestLoadSupportsProcessGroups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: api
    command: "go run ./cmd/api"
  - name: web
    command: "pnpm dev"
groups:
  - name: dev
    processes: [api, web]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	project, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	group, err := project.Group("dev")
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	if len(group.Processes) != 2 {
		t.Fatalf("unexpected group member count: %d", len(group.Processes))
	}
	if group.Processes[0] != "api" || group.Processes[1] != "web" {
		t.Fatalf("unexpected group members: %#v", group.Processes)
	}
}

func TestLoadRejectsGroupWithUnknownProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: api
    command: "go run ./cmd/api"
groups:
  - name: dev
    processes: [api, web]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for unknown group process")
	}
}

func TestLoadRejectsGroupNameConflictWithProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: dev
    command: "go run ./cmd/api"
groups:
  - name: dev
    processes: [dev]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for group/process name conflict")
	}
}

func TestSavePersistsGroups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	projectCfg := model.Config{
		Version: model.CurrentVersion,
		Defaults: model.Defaults{
			StopTimeoutSec: model.DefaultStopTimeout,
			Shell:          model.DefaultShell,
		},
		Processes: []model.Process{
			{Name: "api", Command: "go run ./cmd/api"},
			{Name: "web", Command: "pnpm dev"},
		},
		Groups: []model.ProcessGroup{
			{Name: "dev", Processes: []string{"api", "web"}},
		},
	}

	project, err := Save(cfgPath, projectCfg)
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if len(project.Groups) != 1 || project.Groups[0].Name != "dev" {
		t.Fatalf("unexpected saved groups: %#v", project.Groups)
	}

	reloaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Groups) != 1 || reloaded.Groups[0].Name != "dev" {
		t.Fatalf("unexpected reloaded groups: %#v", reloaded.Groups)
	}
}
