package gitwt

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	branchPrefix = "refs/heads/"
)

type Worktree struct {
	Name           string
	Dir            string
	Branch         string
	Head           string
	Bare           bool
	Detached       bool
	Prunable       bool
	PrunableReason string
}

func Discover(repoRoot string) ([]Worktree, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	cmd := exec.Command("git", "-C", root, "worktree", "list", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return nil, fmt.Errorf("list git worktrees: %w", err)
		}
		return nil, fmt.Errorf("list git worktrees: %s", msg)
	}

	items, err := parsePorcelain(out)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no git worktrees found")
	}

	// Mark prunable worktrees: duplicate dirs or missing directories.
	seen := map[string]bool{}
	for i := range items {
		dir := items[i].Dir
		if seen[dir] {
			items[i].Prunable = true
			if items[i].PrunableReason == "" {
				items[i].PrunableReason = "duplicate entry"
			}
		} else {
			seen[dir] = true
			if !items[i].Bare && !items[i].Prunable {
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					items[i].Prunable = true
					if items[i].PrunableReason == "" {
						items[i].PrunableReason = "directory not found"
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Dir < items[j].Dir
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func Resolve(items []Worktree, selector string) (*Worktree, error) {
	sel := strings.TrimSpace(selector)
	if sel == "" {
		return nil, fmt.Errorf("worktree selector is required")
	}

	if abs, err := filepath.Abs(sel); err == nil {
		cleanAbs := filepath.Clean(abs)
		for i := range items {
			if filepath.Clean(items[i].Dir) == cleanAbs {
				return &items[i], nil
			}
		}
	}

	matches := make([]int, 0, 1)
	for i := range items {
		if items[i].Name == sel {
			matches = append(matches, i)
		}
	}
	if len(matches) == 1 {
		return &items[matches[0]], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("worktree %q is ambiguous; use full path", sel)
	}
	return nil, fmt.Errorf("worktree %q not found", sel)
}

func parsePorcelain(data []byte) ([]Worktree, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	items := []Worktree{}
	var current *Worktree

	flush := func() error {
		if current == nil {
			return nil
		}
		current.Dir = strings.TrimSpace(current.Dir)
		if current.Dir == "" {
			return fmt.Errorf("invalid git worktree output: missing worktree path")
		}
		abs, err := filepath.Abs(current.Dir)
		if err != nil {
			return fmt.Errorf("resolve worktree path %q: %w", current.Dir, err)
		}
		current.Dir = filepath.Clean(abs)
		current.Name = filepath.Base(current.Dir)
		if current.Name == "." || current.Name == string(filepath.Separator) || current.Name == "" {
			current.Name = current.Dir
		}
		current.Branch = strings.TrimPrefix(current.Branch, branchPrefix)
		items = append(items, *current)
		current = nil
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &Worktree{Dir: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch "))
		case line == "bare":
			current.Bare = true
		case line == "detached":
			current.Detached = true
		case strings.HasPrefix(line, "prunable "):
			current.Prunable = true
			current.PrunableReason = strings.TrimSpace(strings.TrimPrefix(line, "prunable "))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse git worktree output: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return items, nil
}
