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

	"github.com/xrehpicx/wts/internal/model"
)

const DefaultConfigFile = ".wts.yaml"

var processNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
	if err := normalizeAndValidate(&cfg); err != nil {
		return nil, err
	}

	return model.NewProject(absPath, rootDir, cfg), nil
}

func normalizeAndValidate(cfg *model.Config) error {
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

	if len(cfg.Processes) == 0 {
		return fmt.Errorf("processes must contain at least one process")
	}

	seen := make(map[string]struct{}, len(cfg.Processes))
	for i := range cfg.Processes {
		proc := &cfg.Processes[i]
		if err := normalizeProcess(proc); err != nil {
			return fmt.Errorf("process[%d]: %w", i, err)
		}
		if _, exists := seen[proc.Name]; exists {
			return fmt.Errorf("duplicate process name %q", proc.Name)
		}
		seen[proc.Name] = struct{}{}
	}

	return nil
}

func normalizeProcess(proc *model.Process) error {
	proc.Name = strings.TrimSpace(proc.Name)
	if proc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !processNamePattern.MatchString(proc.Name) {
		return fmt.Errorf("name %q is invalid (allowed: letters, numbers, '.', '_', '-')", proc.Name)
	}

	proc.Command = strings.TrimSpace(proc.Command)
	if proc.Command == "" {
		return fmt.Errorf("command is required")
	}

	if proc.Env == nil {
		proc.Env = map[string]string{}
	}
	return nil
}
