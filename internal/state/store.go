package state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CurrentVersion   = 1
	DefaultDirName   = ".workswitch"
	DefaultStateFile = "state.yaml"
)

type File struct {
	Version int         `yaml:"version"`
	Repos   []RepoState `yaml:"repos"`
}

type RepoState struct {
	Root      string     `yaml:"root"`
	Selected  int        `yaml:"selected,omitempty"`
	Worktrees []Worktree `yaml:"worktrees"`
}

type Worktree struct {
	Name    string `yaml:"name"`
	Dir     string `yaml:"dir"`
	Process string `yaml:"process"`
	Group   string `yaml:"group,omitempty"`
}

type Store struct {
	Path string
}

func NewStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		path = filepath.Join(home, DefaultDirName, DefaultStateFile)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve state path: %w", err)
	}
	return &Store{Path: abs}, nil
}

func (s *Store) Load() (*File, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Version: CurrentVersion, Repos: []RepoState{}}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	f := File{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	if f.Version != CurrentVersion {
		return nil, fmt.Errorf("unsupported state version %d", f.Version)
	}
	if f.Repos == nil {
		f.Repos = []RepoState{}
	}
	return &f, nil
}

func (s *Store) Save(f *File) error {
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	if f.Version != CurrentVersion {
		return fmt.Errorf("unsupported state version %d", f.Version)
	}

	for i := range f.Repos {
		repo := &f.Repos[i]
		if err := normalizeRepo(repo); err != nil {
			return fmt.Errorf("repo[%d]: %w", i, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	payload, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode state yaml: %w", err)
	}
	if err := os.WriteFile(s.Path, payload, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}

func (f *File) EnsureRepo(root string) (*RepoState, int, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, -1, fmt.Errorf("resolve repo root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	for i := range f.Repos {
		if filepath.Clean(f.Repos[i].Root) == absRoot {
			return &f.Repos[i], i, nil
		}
	}

	f.Repos = append(f.Repos, RepoState{Root: absRoot, Worktrees: []Worktree{}})
	idx := len(f.Repos) - 1
	return &f.Repos[idx], idx, nil
}

func (r *RepoState) Worktree(name string) (*Worktree, int) {
	for i := range r.Worktrees {
		if r.Worktrees[i].Name == name {
			return &r.Worktrees[i], i
		}
	}
	return nil, -1
}

func (r *RepoState) RemoveWorktree(name string) bool {
	_, idx := r.Worktree(name)
	if idx < 0 {
		return false
	}
	r.Worktrees = append(r.Worktrees[:idx], r.Worktrees[idx+1:]...)
	if r.Selected >= len(r.Worktrees) {
		r.Selected = max(0, len(r.Worktrees)-1)
	}
	return true
}

func (r *RepoState) Normalize() error {
	return normalizeRepo(r)
}

func normalizeRepo(r *RepoState) error {
	r.Root = strings.TrimSpace(r.Root)
	if r.Root == "" {
		return fmt.Errorf("root is required")
	}

	seen := map[string]struct{}{}
	for i := range r.Worktrees {
		w := &r.Worktrees[i]
		w.Name = strings.TrimSpace(w.Name)
		w.Dir = strings.TrimSpace(w.Dir)
		w.Process = strings.TrimSpace(w.Process)
		w.Group = strings.TrimSpace(w.Group)
		if w.Name == "" {
			return fmt.Errorf("worktree[%d].name is required", i)
		}
		if w.Dir == "" {
			return fmt.Errorf("worktree[%d].dir is required", i)
		}
		if w.Process == "" {
			return fmt.Errorf("worktree[%d].process is required", i)
		}
		if _, ok := seen[w.Name]; ok {
			return fmt.Errorf("duplicate worktree name %q", w.Name)
		}
		seen[w.Name] = struct{}{}
	}

	sort.Slice(r.Worktrees, func(i, j int) bool {
		return r.Worktrees[i].Name < r.Worktrees[j].Name
	})
	if r.Selected < 0 {
		r.Selected = 0
	}
	if len(r.Worktrees) == 0 {
		r.Selected = 0
	} else if r.Selected >= len(r.Worktrees) {
		r.Selected = len(r.Worktrees) - 1
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
