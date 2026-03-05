package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xrehpicx/wts/internal/model"
)

func TestLoadAssignsDefaultsAndImplicitGroup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatalf("mkdir api: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web: %v", err)
	}

	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 1
workspaces:
  - name: api
    dir: ./api
    command: "go run ."
    group: backend
  - name: web
    dir: ./web
    command: "pnpm dev"
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

	web, err := project.Workspace("web")
	if err != nil {
		t.Fatalf("workspace web: %v", err)
	}
	if web.EffectiveGroup != model.ImplicitGroupPrefix+"web" {
		t.Fatalf("unexpected implicit group: got %q", web.EffectiveGroup)
	}
	if web.ResolvedDir != webDir {
		t.Fatalf("unexpected resolved dir: got %q", web.ResolvedDir)
	}
}

func TestLoadRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}

	cfgPath := filepath.Join(dir, DefaultConfigFile)
	content := `version: 2
workspaces:
  - name: app
    dir: ./app
    command: "go run ."
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for invalid version")
	}
}
