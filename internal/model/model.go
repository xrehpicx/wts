package model

import (
	"fmt"
	"sort"
)

const (
	CurrentVersion      = 1
	DefaultStopTimeout  = 8
	DefaultShell        = "/bin/sh"
	ImplicitGroupPrefix = "__self__:"
)

type Config struct {
	Version    int         `yaml:"version"`
	Defaults   Defaults    `yaml:"defaults"`
	Workspaces []Workspace `yaml:"workspaces"`
}

type Defaults struct {
	StopTimeoutSec int    `yaml:"stop_timeout_sec"`
	Shell          string `yaml:"shell"`
}

type Workspace struct {
	Name           string            `yaml:"name"`
	Dir            string            `yaml:"dir"`
	Command        string            `yaml:"command"`
	Group          string            `yaml:"group,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`
	ResolvedDir    string            `yaml:"-"`
	EffectiveGroup string            `yaml:"-"`
}

type Project struct {
	ConfigPath string
	RootDir    string
	Defaults   Defaults
	Workspaces []Workspace

	indexByName map[string]int
}

func NewProject(configPath, rootDir string, cfg Config) *Project {
	p := &Project{
		ConfigPath:  configPath,
		RootDir:     rootDir,
		Defaults:    cfg.Defaults,
		Workspaces:  cfg.Workspaces,
		indexByName: make(map[string]int, len(cfg.Workspaces)),
	}
	for i := range p.Workspaces {
		p.indexByName[p.Workspaces[i].Name] = i
	}
	return p
}

func (p *Project) Workspace(name string) (*Workspace, error) {
	idx, ok := p.indexByName[name]
	if !ok {
		return nil, fmt.Errorf("workspace %q not found", name)
	}
	return &p.Workspaces[idx], nil
}

func (p *Project) WorkspaceNames() []string {
	names := make([]string, 0, len(p.Workspaces))
	for _, ws := range p.Workspaces {
		names = append(names, ws.Name)
	}
	sort.Strings(names)
	return names
}

func (p *Project) Groups() []string {
	seen := make(map[string]struct{})
	for _, ws := range p.Workspaces {
		seen[ws.EffectiveGroup] = struct{}{}
	}
	groups := make([]string, 0, len(seen))
	for group := range seen {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func (p *Project) IsKnownWorkspace(name string) bool {
	_, ok := p.indexByName[name]
	return ok
}
