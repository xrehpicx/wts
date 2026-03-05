package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/xrehpicx/wks/internal/model"
)

const DefaultConfigFile = ".workswitch.yaml"

var workspaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func Load(configPath string) (*model.Project, error) {
	if configPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve working directory: %w", err)
		}
		configPath = filepath.Join(cwd, DefaultConfigFile)
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", absPath)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := model.Config{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	rootDir := filepath.Dir(absPath)
	if err := normalizeAndValidate(&cfg, rootDir); err != nil {
		return nil, err
	}

	return model.NewProject(absPath, rootDir, cfg), nil
}

func normalizeAndValidate(cfg *model.Config, rootDir string) error {
	if cfg.Version != model.CurrentVersion {
		return fmt.Errorf("unsupported config version %d (expected %d)", cfg.Version, model.CurrentVersion)
	}

	if cfg.Defaults.StopTimeoutSec == 0 {
		cfg.Defaults.StopTimeoutSec = model.DefaultStopTimeout
	}
	if cfg.Defaults.StopTimeoutSec < 1 || cfg.Defaults.StopTimeoutSec > 120 {
		return fmt.Errorf("defaults.stop_timeout_sec must be between 1 and 120")
	}
	if strings.TrimSpace(cfg.Defaults.Shell) == "" {
		cfg.Defaults.Shell = model.DefaultShell
	}

	if len(cfg.Workspaces) == 0 {
		return fmt.Errorf("workspaces must contain at least one workspace")
	}

	seen := make(map[string]struct{}, len(cfg.Workspaces))
	for i := range cfg.Workspaces {
		ws := &cfg.Workspaces[i]
		if err := normalizeWorkspace(ws, rootDir); err != nil {
			return fmt.Errorf("workspace[%d]: %w", i, err)
		}
		if _, exists := seen[ws.Name]; exists {
			return fmt.Errorf("duplicate workspace name %q", ws.Name)
		}
		seen[ws.Name] = struct{}{}
	}

	return nil
}

func normalizeWorkspace(ws *model.Workspace, rootDir string) error {
	ws.Name = strings.TrimSpace(ws.Name)
	if ws.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !workspaceNamePattern.MatchString(ws.Name) {
		return fmt.Errorf("name %q is invalid (allowed: letters, numbers, '.', '_', '-')", ws.Name)
	}

	ws.Command = strings.TrimSpace(ws.Command)
	if ws.Command == "" {
		return fmt.Errorf("command is required")
	}

	ws.Dir = strings.TrimSpace(ws.Dir)
	if ws.Dir == "" {
		return fmt.Errorf("dir is required")
	}

	resolved := ws.Dir
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(rootDir, resolved)
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("dir %q does not exist", ws.Dir)
		}
		return fmt.Errorf("read dir %q: %w", ws.Dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("dir %q is not a directory", ws.Dir)
	}
	ws.ResolvedDir = resolved

	ws.Group = strings.TrimSpace(ws.Group)
	if ws.Group == "" {
		ws.EffectiveGroup = model.ImplicitGroupPrefix + ws.Name
	} else {
		ws.EffectiveGroup = ws.Group
	}

	if ws.Env == nil {
		ws.Env = map[string]string{}
	}

	return nil
}
