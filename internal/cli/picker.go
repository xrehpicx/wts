package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type Picker struct {
	Input    io.Reader
	Output   io.Writer
	ErrOut   io.Writer
	LookPath func(file string) (string, error)
	RunFZF   func(path string, names []string, stderr io.Writer) (string, error)
}

func NewPicker(in io.Reader, out, errOut io.Writer) *Picker {
	return &Picker{
		Input:    in,
		Output:   out,
		ErrOut:   errOut,
		LookPath: exec.LookPath,
		RunFZF:   runFZF,
	}
}

func (p *Picker) Select(names []string) (string, error) {
	if len(names) == 0 {
		return "", fmt.Errorf("no worktrees configured")
	}

	if p.LookPath != nil && p.RunFZF != nil {
		if path, err := p.LookPath("fzf"); err == nil {
			selected, runErr := p.RunFZF(path, names, p.ErrOut)
			if runErr == nil && selected != "" {
				return selected, nil
			}
		}
	}

	_, _ = fmt.Fprintln(p.Output, "Select worktree:")
	for i, name := range names {
		_, _ = fmt.Fprintf(p.Output, "  %d) %s\n", i+1, name)
	}
	_, _ = fmt.Fprint(p.Output, "> ")

	line, err := bufio.NewReader(p.Input).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read selection: %w", err)
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

func runFZF(path string, names []string, stderr io.Writer) (string, error) {
	cmd := exec.Command(path, "--prompt", "worktree> ")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
