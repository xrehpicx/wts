package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolatedClient(t *testing.T) (*Client, string, string) {
	t.Helper()
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	// Keep the socket path below Unix socket limits, including on macOS.
	dir, err := os.MkdirTemp("", "wts-tmux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "socket")
	client := NewClient(bin)
	client.runner = runnerFunc(func(ctx context.Context, name string, args ...string) (string, error) {
		if name == bin {
			args = append([]string{"-S", socket, "-f", "/dev/null"}, args...)
		}
		return (execRunner{}).Run(ctx, name, args...)
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = client.runner.Run(ctx, bin, "kill-server")
	})
	return client, bin, dir
}

func waitForFile(ctx context.Context, t *testing.T, path, want string) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if data, err := os.ReadFile(path); err == nil && string(data) == want {
			return
		}
		select {
		case <-ctx.Done():
			data, _ := os.ReadFile(path)
			t.Fatalf("%s = %q, want %q: %v", path, data, want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestClientRunsCompoundCommandInIsolatedTmux(t *testing.T) {
	t.Parallel()
	client, bin, dir := isolatedClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.EnsureSession(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.runner.Run(ctx, bin, "set-option", "-g", "default-shell", "/bin/sh"); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "finished")
	command := `printf '  first\n'; printf 'second\n'; printf done > ` + shellQuote(marker)
	if err := client.StartWindowCommand(ctx, "test", "process", dir, "/bin/sh", command, nil, ProcessPaneTitle("api")); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "done" {
			break
		}
		select {
		case <-ctx.Done():
			logCtx, logCancel := context.WithTimeout(context.Background(), time.Second)
			logs, _ := client.CapturePane(logCtx, "test", "process", 20)
			logCancel()
			t.Fatalf("command did not complete: %v; logs: %s", ctx.Err(), logs)
		case <-ticker.C:
		}
	}
	logs, err := client.CapturePane(ctx, "test", "process", 20)
	if err != nil || !strings.Contains(logs, "  first\nsecond\n") {
		t.Fatalf("capture = %q, %v; expected both commands and indentation", logs, err)
	}
	panes, err := client.ListPanes(ctx, "test", "process")
	if err != nil || len(panes) != 1 || panes[0].Process != "api" {
		t.Fatalf("pane identity = %#v, %v", panes, err)
	}
	if err := client.StopWindow(ctx, "test", "process", time.Second); err != nil {
		t.Fatal(err)
	}
	if exists, err := client.HasWindow(ctx, "test", "process"); err != nil || exists {
		t.Fatalf("window still exists after stop: %v, %v", exists, err)
	}
}

// The user's default shell must not matter: commands never pass through it.
// Many processes share one detached window, and exit is reported promptly.
func TestClientRunsManyPanesIndependentlyOfDefaultShell(t *testing.T) {
	t.Parallel()
	client, bin, dir := isolatedClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.EnsureSession(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	falseShell, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false is not installed")
	}
	if _, err := client.runner.Run(ctx, bin, "set-option", "-g", "default-shell", falseShell); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{"VALUE": `C:\path\`}
	for i := 0; i < 7; i++ {
		marker := filepath.Join(dir, fmt.Sprintf("pane-%d", i))
		command := `printf '%s' "$VALUE" > ` + shellQuote(marker) + `; exec sleep 30`
		start := client.SplitWindowCommand
		if i == 0 {
			start = client.StartWindowCommand
		}
		if err := start(ctx, "test", "many", dir, "/bin/sh", command, env, ProcessPaneTitle(fmt.Sprintf("p%d", i))); err != nil {
			t.Fatalf("start pane %d: %v", i, err)
		}
		waitForFile(ctx, t, marker, env["VALUE"])
	}
	panes, err := client.ListPanes(ctx, "test", "many")
	if err != nil || len(panes) != 7 {
		t.Fatalf("panes = %d, %v", len(panes), err)
	}
	for _, pane := range panes {
		if !pane.Direct || client.ProcessExited(ctx, pane) {
			t.Fatalf("expected a live direct pane: %#v", pane)
		}
	}

	if err := client.StopPane(ctx, panes[0].ID, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := client.StopWindow(ctx, "test", "many", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if exists, err := client.HasWindow(ctx, "test", "many"); err != nil || exists {
		t.Fatalf("window still exists after stop: %v, %v", exists, err)
	}
}
