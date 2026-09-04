package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientRunsCompoundCommandInIsolatedTmux(t *testing.T) {
	t.Parallel()
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
