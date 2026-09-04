package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/xrehpicx/wts/internal/model"
)

const DefaultConfigFile = ".wts.yaml"

var processNamePattern = regexp.MustCompile(`^[^\p{Cc}\p{Cf}]+$`)
var environmentNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

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
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
		return nil, fmt.Errorf("config must contain exactly one YAML document")
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
	prepared, err := normalizedConfig(cfg)
	if err != nil {
		return nil, err
	}
	data, err := marshalNormalized(prepared)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(absPath, data); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	return model.NewProject(absPath, filepath.Dir(absPath), prepared), nil
}

// Marshal validates cfg and returns it as a single YAML document.
func Marshal(cfg model.Config) ([]byte, error) {
	prepared, err := normalizedConfig(cfg)
	if err != nil {
		return nil, err
	}
	return marshalNormalized(prepared)
}

func marshalNormalized(cfg model.Config) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&cfg); err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("finalize yaml: %w", err)
	}
	return buf.Bytes(), nil
}

func normalizedConfig(cfg model.Config) (model.Config, error) {
	prepared := cloneConfig(cfg)
	if err := normalizeAndValidate(&prepared); err != nil {
		return model.Config{}, err
	}
	return prepared, nil
}

func cloneConfig(cfg model.Config) model.Config {
	clone := cfg
	clone.Processes = make([]model.Process, len(cfg.Processes))
	for i, process := range cfg.Processes {
		clone.Processes[i] = process
		if process.Env != nil {
			clone.Processes[i].Env = make(map[string]string, len(process.Env))
			for key, value := range process.Env {
				clone.Processes[i].Env[key] = value
			}
		}
	}
	clone.Groups = make([]model.ProcessGroup, len(cfg.Groups))
	for i, group := range cfg.Groups {
		clone.Groups[i] = group
		clone.Groups[i].Processes = append([]string(nil), group.Processes...)
	}
	return clone
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()

	if info, statErr := os.Stat(path); statErr == nil {
		if err := temp.Chmod(info.Mode().Perm()); err != nil {
			_ = temp.Close()
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		_ = temp.Close()
		return statErr
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
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
	cfg.Defaults.Shell = strings.TrimSpace(cfg.Defaults.Shell)
	if cfg.Defaults.Shell == "" {
		cfg.Defaults.Shell = model.DefaultShell
	}
	if strings.ContainsRune(cfg.Defaults.Shell, '\x00') {
		return fmt.Errorf("defaults.shell must not contain NUL characters")
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
		return fmt.Errorf("name %q is invalid (control and formatting characters are not allowed)", proc.Name)
	}

	proc.Command = strings.TrimSpace(proc.Command)
	if proc.Command == "" {
		return fmt.Errorf("command is required")
	}

	if strings.ContainsRune(proc.Command, '\x00') {
		return fmt.Errorf("command must not contain NUL characters")
	}

	if proc.Env == nil {
		proc.Env = map[string]string{}
	}
	for key, value := range proc.Env {
		if !environmentNamePattern.MatchString(key) {
			return fmt.Errorf("environment variable name %q is invalid", key)
		}
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("environment variable %q must not contain NUL characters", key)
		}
	}
	return nil
}

func normalizeGroup(group *model.ProcessGroup, processes map[string]struct{}) error {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !processNamePattern.MatchString(group.Name) {
		return fmt.Errorf("name %q is invalid (control and formatting characters are not allowed)", group.Name)
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
