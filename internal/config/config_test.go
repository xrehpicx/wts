package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestLoadRejectsInvalidEnvironmentVariableName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: dev
    command: go run .
    env:
      "BAD; touch /tmp/wts-injected": value
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "environment variable") {
		t.Fatalf("expected invalid environment variable error, got %v", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
processes:
  - name: dev
    command: go run .
---
version: 1
processes:
  - name: ignored
    command: echo ignored
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "one YAML document") {
		t.Fatalf("expected multiple document error, got %v", err)
	}
}

func TestMarshalProducesRoundTrippableYAML(t *testing.T) {
	t.Parallel()

	cfg := model.Config{
		Version: model.CurrentVersion,
		Defaults: model.Defaults{
			StopTimeoutSec: model.DefaultStopTimeout,
			Shell:          model.DefaultShell,
		},
		Processes: []model.Process{
			{
				Name:    "web: dev",
				Command: `printf '%s\n' "value: #1"`,
				Env:     map[string]string{"GREETING": "hello: #world"},
			},
		},
	}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(data, []byte("web: dev")) {
		t.Fatalf("expected process name in YAML, got %q", data)
	}

	path := filepath.Join(t.TempDir(), DefaultConfigFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write marshaled config: %v", err)
	}
	project, err := Load(path)
	if err != nil {
		t.Fatalf("load marshaled config: %v", err)
	}
	if got := project.Processes[0].Command; got != cfg.Processes[0].Command {
		t.Fatalf("command changed after round trip: got %q; want %q", got, cfg.Processes[0].Command)
	}
}

func TestProcessNameAllowsShellPunctuationButRejectsControlCharacters(t *testing.T) {
	t.Parallel()

	valid := model.Config{
		Version: model.CurrentVersion,
		Processes: []model.Process{{
			Name:    "dev; preview (local)",
			Command: "echo ok",
		}},
	}
	if _, err := Marshal(valid); err != nil {
		t.Fatalf("valid display name rejected: %v", err)
	}

	invalid := valid
	invalid.Processes[0].Name = "dev\npreview"
	if _, err := Marshal(invalid); err == nil {
		t.Fatal("expected control character in process name to be rejected")
	}
}

func TestSaveNewConfigUsesPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	t.Parallel()

	path := filepath.Join(t.TempDir(), DefaultConfigFile)
	cfg := model.Config{
		Version:   model.CurrentVersion,
		Processes: []model.Process{{Name: "dev", Command: "echo ok"}},
	}
	if _, err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("new config mode = %o; want 600", got)
	}
}

func TestMarshalRejectsNULInProcessExecutionValues(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"shell", "command", "environment"} {
		t.Run(field, func(t *testing.T) {
			cfg := model.Config{Version: model.CurrentVersion, Processes: []model.Process{{Name: "api", Command: "echo ok"}}}
			switch field {
			case "shell":
				cfg.Defaults.Shell = "/bin/sh\x00"
			case "command":
				cfg.Processes[0].Command = "echo\x00ok"
			case "environment":
				cfg.Processes[0].Env = map[string]string{"VALUE": "bad\x00value"}
			}
			if _, err := Marshal(cfg); err == nil {
				t.Fatal("expected NUL validation error before invoking tmux")
			}
		})
	}
}

func TestSavePreservesFileAndCallerOnValidationFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), DefaultConfigFile)
	original := []byte("existing config\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	cfg := model.Config{Version: model.CurrentVersion, Processes: []model.Process{{Name: " api ", Command: " "}}}
	if _, err := Save(path, cfg); err == nil {
		t.Fatal("expected invalid command error")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("failed save changed original: %q, %v", data, err)
	}
	if cfg.Processes[0].Name != " api " {
		t.Fatal("failed save mutated caller configuration")
	}
}
