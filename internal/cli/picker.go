package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// ErrSelectionCanceled indicates that the user dismissed the selector.
var ErrSelectionCanceled = errors.New("selection canceled")

// Picker selects a worktree using fzf when installed or a numbered prompt.
type Picker struct {
	Input    io.Reader
	Output   io.Writer
	ErrOut   io.Writer
	LookPath func(file string) (string, error)
	RunFZF   func(ctx context.Context, path string, names []string, stderr io.Writer) (string, error)
}

// NewPicker creates a selector using the supplied streams.
func NewPicker(in io.Reader, out, errOut io.Writer) *Picker {
	return &Picker{Input: in, Output: out, ErrOut: errOut, LookPath: exec.LookPath, RunFZF: runFZF}
}

// Select chooses an item without a cancellation deadline.
func (p *Picker) Select(names []string) (string, error) {
	return p.SelectContext(context.Background(), names)
}

// SelectContext chooses an item, honoring cancellation when running fzf.
func (p *Picker) SelectContext(ctx context.Context, names []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no git worktrees found (create one with: git worktree add ../branch-name)")
	}
	if p.LookPath != nil && p.RunFZF != nil {
		if path, err := p.LookPath("fzf"); err == nil {
			selected, err := p.RunFZF(ctx, path, names, p.ErrOut)
			if err != nil {
				return "", err
			}
			if !slices.Contains(names, selected) {
				return "", fmt.Errorf("selector returned an unknown worktree %q", selected)
			}
			return selected, nil
		}
	}

	if _, err := fmt.Fprintln(p.Output, "Select worktree:"); err != nil {
		return "", err
	}
	for i, name := range names {
		if _, err := fmt.Fprintf(p.Output, "  %d) %s\n", i+1, displayField(name)); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprint(p.Output, "> "); err != nil {
		return "", err
	}

	line, err := bufio.NewReader(p.Input).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || len(line) == 0) {
		return "", fmt.Errorf("read selection: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	index, err := strconv.Atoi(line)
	if err != nil {
		return "", fmt.Errorf("invalid selection %q", line)
	}
	if index < 1 || index > len(names) {
		return "", fmt.Errorf("selection out of range")
	}
	return names[index-1], nil
}

func runFZF(ctx context.Context, path string, names []string, stderr io.Writer) (string, error) {
	cmd := exec.CommandContext(ctx, path, "--prompt", "worktree> ", "--read0", "--print0", "--no-multi")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\x00") + "\x00")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return "", ErrSelectionCanceled
		}
		return "", fmt.Errorf("run fzf: %w", err)
	}
	return strings.TrimSuffix(stdout.String(), "\x00"), nil
}
