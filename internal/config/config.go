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

var processNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._:@/ -]+$`)

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

func Save(configPath string, cfg model.Config) (*model.Project, error) {
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
	if err := normalizeAndValidate(&cfg); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&cfg); err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("finalize yaml: %w", err)
	}
	if err := os.WriteFile(absPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	return model.NewProject(absPath, filepath.Dir(absPath), cfg), nil
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

	groupSeen := make(map[string]struct{}, len(cfg.Groups))
	for i := range cfg.Groups {
		group := &cfg.Groups[i]
		if err := normalizeGroup(group, seen); err != nil {
			return fmt.Errorf("groups[%d]: %w", i, err)
		}
		if _, exists := seen[group.Name]; exists {
			return fmt.Errorf("group name %q conflicts with a process name", group.Name)
		}
		if _, exists := groupSeen[group.Name]; exists {
			return fmt.Errorf("duplicate group name %q", group.Name)
		}
		groupSeen[group.Name] = struct{}{}
	}

	return nil
}

func normalizeProcess(proc *model.Process) error {
	proc.Name = strings.TrimSpace(proc.Name)
	if proc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !processNamePattern.MatchString(proc.Name) {
		return fmt.Errorf("name %q is invalid (allowed: letters, numbers, '.', '_', '-', ':', '@', '/', ' ')", proc.Name)
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

func normalizeGroup(group *model.ProcessGroup, processes map[string]struct{}) error {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !processNamePattern.MatchString(group.Name) {
		return fmt.Errorf("name %q is invalid (allowed: letters, numbers, '.', '_', '-', ':', '@', '/', ' ')", group.Name)
	}
	if len(group.Processes) == 0 {
		return fmt.Errorf("processes must contain at least one process name")
	}

	seenMembers := make(map[string]struct{}, len(group.Processes))
	for i := range group.Processes {
		name := strings.TrimSpace(group.Processes[i])
		if name == "" {
			return fmt.Errorf("processes[%d]: name is required", i)
		}
		if _, ok := processes[name]; !ok {
			return fmt.Errorf("processes[%d]: unknown process %q", i, name)
		}
		if _, exists := seenMembers[name]; exists {
			return fmt.Errorf("processes[%d]: duplicate process %q", i, name)
		}
		seenMembers[name] = struct{}{}
		group.Processes[i] = name
	}

	return nil
}
