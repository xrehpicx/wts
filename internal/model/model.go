package model

import "fmt"

const (
	CurrentVersion     = 1
	DefaultStopTimeout = 8
	DefaultShell       = "/bin/sh"
)

type Config struct {
	Version   int            `yaml:"version"`
	Defaults  Defaults       `yaml:"defaults"`
	Processes []Process      `yaml:"processes"`
	Groups    []ProcessGroup `yaml:"groups,omitempty"`
}

type Defaults struct {
	StopTimeoutSec int    `yaml:"stop_timeout_sec"`
	Shell          string `yaml:"shell"`
}

type Process struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type ProcessGroup struct {
	Name      string   `yaml:"name"`
	Processes []string `yaml:"processes"`
}

type TargetKind string

const (
	TargetProcess TargetKind = "process"
	TargetGroup   TargetKind = "group"
)

type Target struct {
	Kind         TargetKind
	Name         string
	ProcessNames []string
}

type Project struct {
	ConfigPath string
	RootDir    string
	Defaults   Defaults
	Processes  []Process
	Groups     []ProcessGroup

	procByName  map[string]int
	groupByName map[string]int
}

func NewProject(configPath, rootDir string, cfg Config) *Project {
	p := &Project{
		ConfigPath:  configPath,
		RootDir:     rootDir,
		Defaults:    cfg.Defaults,
		Processes:   cfg.Processes,
		Groups:      cfg.Groups,
		procByName:  make(map[string]int, len(cfg.Processes)),
		groupByName: make(map[string]int, len(cfg.Groups)),
	}
	for i := range p.Processes {
		p.procByName[p.Processes[i].Name] = i
	}
	for i := range p.Groups {
		p.groupByName[p.Groups[i].Name] = i
	}
	return p
}

func (p *Project) Process(name string) (*Process, error) {
	idx, ok := p.procByName[name]
	if !ok {
		return nil, fmt.Errorf("process %q not found", name)
	}
	return &p.Processes[idx], nil
}

func (p *Project) ProcessNames() []string {
	names := make([]string, 0, len(p.Processes))
	for _, proc := range p.Processes {
		names = append(names, proc.Name)
	}
	return names
}

func (p *Project) Group(name string) (*ProcessGroup, error) {
	idx, ok := p.groupByName[name]
	if !ok {
		return nil, fmt.Errorf("group %q not found", name)
	}
	return &p.Groups[idx], nil
}

func (p *Project) GroupNames() []string {
	names := make([]string, 0, len(p.Groups))
	for _, group := range p.Groups {
		names = append(names, group.Name)
	}
	return names
}

func (p *Project) ResolveTarget(processName, groupName string) (Target, error) {
	if processName != "" && groupName != "" {
		return Target{}, fmt.Errorf("only one of process or group may be selected")
	}

	if groupName != "" {
		group, err := p.Group(groupName)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind:         TargetGroup,
			Name:         group.Name,
			ProcessNames: append([]string(nil), group.Processes...),
		}, nil
	}

	if processName == "" {
		if len(p.Processes) == 0 {
			return Target{}, fmt.Errorf("no process profiles configured")
		}
		processName = p.Processes[0].Name
	}

	if _, err := p.Process(processName); err != nil {
		return Target{}, err
	}
	return Target{
		Kind:         TargetProcess,
		Name:         processName,
		ProcessNames: []string{processName},
	}, nil
}

func (p *Project) Targets() []Target {
	targets := make([]Target, 0, len(p.Processes)+len(p.Groups))
	for _, process := range p.Processes {
		targets = append(targets, Target{
			Kind:         TargetProcess,
			Name:         process.Name,
			ProcessNames: []string{process.Name},
		})
	}
	for _, group := range p.Groups {
		targets = append(targets, Target{
			Kind:         TargetGroup,
			Name:         group.Name,
			ProcessNames: append([]string(nil), group.Processes...),
		})
	}
	return targets
}

func (p *Project) Config() Config {
	cfg := Config{
		Version:  CurrentVersion,
		Defaults: p.Defaults,
	}

	cfg.Processes = make([]Process, 0, len(p.Processes))
	for _, process := range p.Processes {
		env := make(map[string]string, len(process.Env))
		for key, value := range process.Env {
			env[key] = value
		}
		cfg.Processes = append(cfg.Processes, Process{
			Name:    process.Name,
			Command: process.Command,
			Env:     env,
		})
	}

	cfg.Groups = make([]ProcessGroup, 0, len(p.Groups))
	for _, group := range p.Groups {
		cfg.Groups = append(cfg.Groups, ProcessGroup{
			Name:      group.Name,
			Processes: append([]string(nil), group.Processes...),
		})
	}

	return cfg
}
