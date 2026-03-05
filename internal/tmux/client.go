package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Backend interface {
	EnsureTmux(ctx context.Context) error
	EnsureSession(ctx context.Context, session string) error
	HasWindow(ctx context.Context, session, window string) (bool, error)
	StartWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string) error
	StopWindow(ctx context.Context, session, window string, timeout time.Duration) error
	SetSessionOption(ctx context.Context, session, key, value string) error
	GetSessionOption(ctx context.Context, session, key string) (string, error)
	CapturePane(ctx context.Context, session, window string, lines int) (string, error)
	Attach(ctx context.Context, session, window string) error
}

type runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

type Client struct {
	bin    string
	runner runner
}

func NewClient(bin string) *Client {
	if strings.TrimSpace(bin) == "" {
		bin = "tmux"
	}
	return &Client{bin: bin, runner: execRunner{}}
}

func (c *Client) EnsureTmux(ctx context.Context) error {
	if _, err := exec.LookPath(c.bin); err != nil {
		return fmt.Errorf("tmux is required but was not found in PATH")
	}
	_, err := c.runner.Run(ctx, c.bin, "-V")
	if err != nil {
		return fmt.Errorf("tmux not available: %w", err)
	}
	return nil
}

func (c *Client) EnsureSession(ctx context.Context, session string) error {
	_, err := c.runner.Run(ctx, c.bin, "has-session", "-t", session)
	if err == nil {
		return nil
	}
	_, err = c.runner.Run(ctx, c.bin, "new-session", "-d", "-s", session)
	if err != nil {
		return fmt.Errorf("create tmux session %q: %w", session, err)
	}
	return nil
}

func (c *Client) HasWindow(ctx context.Context, session, window string) (bool, error) {
	_, err := c.runner.Run(ctx, c.bin, "list-windows", "-t", session+":"+window)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	if strings.Contains(err.Error(), "can't find window") || strings.Contains(err.Error(), "can't find session") {
		return false, nil
	}
	return false, fmt.Errorf("check window %q in session %q: %w", window, session, err)
}

func (c *Client) StartWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string) error {
	exists, err := c.HasWindow(ctx, session, window)
	if err != nil {
		return err
	}
	if exists {
		if _, err := c.runner.Run(ctx, c.bin, "kill-window", "-t", session+":"+window); err != nil {
			return fmt.Errorf("reset existing window %q: %w", window, err)
		}
	}

	if _, err := c.runner.Run(ctx, c.bin, "new-window", "-d", "-t", session, "-n", window, "-c", dir); err != nil {
		return fmt.Errorf("create window %q: %w", window, err)
	}

	payload := buildPayload(command, env)
	if _, err := c.runner.Run(ctx, c.bin, "send-keys", "-t", session+":"+window, shell+" -lc "+shellQuote(payload), "C-m"); err != nil {
		return fmt.Errorf("start command for workspace window %q: %w", window, err)
	}
	return nil
}

func (c *Client) StopWindow(ctx context.Context, session, window string, timeout time.Duration) error {
	exists, err := c.HasWindow(ctx, session, window)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if _, err := c.runner.Run(ctx, c.bin, "send-keys", "-t", session+":"+window, "C-c"); err != nil {
		return fmt.Errorf("send interrupt to %q: %w", window, err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive, err := c.HasWindow(ctx, session, window)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if _, err := c.runner.Run(ctx, c.bin, "kill-window", "-t", session+":"+window); err != nil {
		return fmt.Errorf("force kill window %q: %w", window, err)
	}
	return nil
}

func (c *Client) SetSessionOption(ctx context.Context, session, key, value string) error {
	if strings.TrimSpace(value) == "" {
		_, err := c.runner.Run(ctx, c.bin, "set-option", "-t", session, "-q", "-u", key)
		if err != nil {
			return fmt.Errorf("unset tmux option %q: %w", key, err)
		}
		return nil
	}
	_, err := c.runner.Run(ctx, c.bin, "set-option", "-t", session, "-q", key, value)
	if err != nil {
		return fmt.Errorf("set tmux option %q: %w", key, err)
	}
	return nil
}

func (c *Client) GetSessionOption(ctx context.Context, session, key string) (string, error) {
	value, err := c.runner.Run(ctx, c.bin, "show-option", "-t", session, "-v", key)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		if strings.Contains(err.Error(), "invalid option") || strings.Contains(err.Error(), "unknown option") {
			return "", nil
		}
		return "", fmt.Errorf("read tmux option %q: %w", key, err)
	}
	return strings.TrimSpace(value), nil
}

func (c *Client) CapturePane(ctx context.Context, session, window string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	output, err := c.runner.Run(ctx, c.bin, "capture-pane", "-p", "-t", session+":"+window, "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return "", fmt.Errorf("capture pane for %q: %w", window, err)
	}
	return output, nil
}

func (c *Client) Attach(ctx context.Context, session, window string) error {
	if _, err := c.runner.Run(ctx, c.bin, "select-window", "-t", session+":"+window); err != nil {
		return fmt.Errorf("select window %q: %w", window, err)
	}
	if os.Getenv("TMUX") != "" {
		if _, err := c.runner.Run(ctx, c.bin, "switch-client", "-t", session); err != nil {
			return fmt.Errorf("switch tmux client to %q: %w", session, err)
		}
		return nil
	}
	if _, err := c.runner.Run(ctx, c.bin, "attach-session", "-t", session); err != nil {
		return fmt.Errorf("attach tmux session %q: %w", session, err)
	}
	return nil
}

func buildPayload(command string, env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("export %s=%s", key, shellQuote(env[key])))
	}
	parts = append(parts, "exec "+command)
	return strings.Join(parts, "; ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
