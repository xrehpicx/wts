package model

import "fmt"

const (
	CurrentVersion     = 1
	DefaultStopTimeout = 8
	DefaultShell       = "/bin/sh"
)

type Config struct {
	Version   int       `yaml:"version"`
	Defaults  Defaults  `yaml:"defaults"`
	Processes []Process `yaml:"processes"`
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

type Project struct {
	ConfigPath string
	RootDir    string
	Defaults   Defaults
	Processes  []Process

	procByName map[string]int
}

func NewProject(configPath, rootDir string, cfg Config) *Project {
	p := &Project{
		ConfigPath: configPath,
		RootDir:    rootDir,
		Defaults:   cfg.Defaults,
		Processes:  cfg.Processes,
		procByName: make(map[string]int, len(cfg.Processes)),
	}
	for i := range p.Processes {
		p.procByName[p.Processes[i].Name] = i
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
