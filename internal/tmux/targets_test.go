package tmux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowTargetsAreSessionQualifiedAndExact(t *testing.T) {
	t.Parallel()
	session := "wts_fea23599_t3code-seed-e2e-members-badge"
	window := "ws_fea23599_t3code-seed-e2e-members-badge"
	if got, want := SessionTarget(session), "="+session+":"; got != want {
		t.Fatalf("SessionTarget() = %q; want %q", got, want)
	}
	if got, want := WindowTarget(session, window), "="+session+":="+window; got != want {
		t.Fatalf("WindowTarget() = %q; want %q", got, want)
	}
}

// TestWindowCommandsNeverTargetBareWindowNames records every tmux -t argument
// and requires each one to be a pane ID or an exact session-qualified target.
// tmux resolves a bare or unqualified name against the current session.
func TestWindowCommandsNeverTargetBareWindowNames(t *testing.T) {
	t.Parallel()
	const session, window = "wts_sess", "ws_win"
	var targets []string
	listWindowsCalls := 0
	client := NewClient("tmux")
	client.runner = runnerFunc(func(_ context.Context, name string, args ...string) (string, error) {
		if name == "pgrep" {
			return "", nil
		}
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-t" {
				targets = append(targets, args[0]+" "+args[i+1])
			}
		}
		switch args[0] {
		case "list-windows":
			listWindowsCalls++
			if listWindowsCalls == 1 {
				// StartWindowCommand: report the window missing so it is created.
				return "", errors.New("can't find window: " + window)
			}
			return "", nil
		case "list-panes":
			return "%3\tapi\twts:api\t123\tsh\t1", nil
		case "split-window":
			return "%4\n", nil
		}
		return "", nil
	})
	ctx := context.Background()

	if err := client.StartWindowCommand(ctx, session, window, "/tmp", "/bin/sh", "true", nil, ProcessPaneTitle("api")); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := client.SplitWindowCommand(ctx, session, window, "/tmp", "/bin/sh", "true", nil, ProcessPaneTitle("web")); err != nil {
		t.Fatalf("split: %v", err)
	}
	if err := client.SetPaneTitle(ctx, session, window, ProcessPaneTitle("api")); err != nil {
		t.Fatalf("set title: %v", err)
	}
	if _, err := client.HasWindow(ctx, session, window); err != nil {
		t.Fatalf("has window: %v", err)
	}
	if _, err := client.ListPanes(ctx, session, window); err != nil {
		t.Fatalf("list panes: %v", err)
	}
	if _, err := client.CapturePane(ctx, session, window, 10); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if _, err := client.PaneCurrentCommand(ctx, session, window); err != nil {
		t.Fatalf("pane command: %v", err)
	}
	if err := client.SetSessionOption(ctx, session, ActiveProcessOptionKey(), "api"); err != nil {
		t.Fatalf("set option: %v", err)
	}
	if _, err := client.GetSessionOption(ctx, session, ActiveProcessOptionKey()); err != nil {
		t.Fatalf("get option: %v", err)
	}
	if err := client.EnsureSession(ctx, session); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if err := client.StopWindow(ctx, session, window, time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}

	windowTarget := WindowTarget(session, window)
	sessionTarget := SessionTarget(session)
	sawWindow := false
	for _, entry := range targets {
		command, target, _ := strings.Cut(entry, " ")
		switch {
		case strings.HasPrefix(target, "%"):
		case target == windowTarget:
			sawWindow = true
		case target == sessionTarget && (command == "new-window" || command == "set-option" || command == "show-option" || command == "has-session"):
		default:
			t.Errorf("%s used target %q; want a pane ID or %q", command, target, windowTarget)
		}
	}
	if !sawWindow {
		t.Fatalf("no window-scoped commands recorded: %v", targets)
	}
}

func TestParseWindowSessionsMatchesExactWindowName(t *testing.T) {
	t.Parallel()
	output := "0\tws_abc_repo-extra\n" +
		"wts_a_repo\tws_abc_repo\n" +
		"wts_a_repo\tws_abc_repo\n" +
		"wts_b_feature\tfish\n" +
		"wts_c_feature\tws_abc_repo\r\n"
	got := parseWindowSessions(output, "ws_abc_repo")
	if strings.Join(got, ",") != "wts_a_repo,wts_c_feature" {
		t.Fatalf("sessions = %v", got)
	}
}

func TestFindWindowSessionsTreatsMissingServerAsEmpty(t *testing.T) {
	t.Parallel()
	client := NewClient("tmux")
	client.runner = runnerFunc(func(context.Context, string, ...string) (string, error) {
		return "", errors.New("no server running on /tmp/tmux-501/default")
	})
	sessions, err := client.FindWindowSessions(context.Background(), "ws_x")
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions = %v, %v; want none", sessions, err)
	}
}

// TestRealTmuxFindsAndStopsWindowOutsideCurrentSession reproduces the
// 2026-10-03 failure on an isolated tmux server: a worktree window lives in
// its own detached session while another session is current. The old lookup
// (the wrong session) reports "can't find window"; the session-qualified
// lookup finds the window and StopWindow kills its process.
func TestRealTmuxFindsAndStopsWindowOutsideCurrentSession(t *testing.T) {
	t.Parallel()
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tmuxRun := func(args ...string) string {
		t.Helper()
		out, err := client.runner.Run(ctx, bin, args...)
		if err != nil {
			t.Fatalf("tmux %s: %v", strings.Join(args, " "), err)
		}
		return out
	}

	const current, session, window = "wts_test_main", "wts_test_x", "ws_test_x"
	tmuxRun("new-session", "-d", "-s", current)
	// A session whose name extends the target's must not satisfy exact lookups.
	tmuxRun("new-session", "-d", "-s", session+"_extra")
	tmuxRun("new-session", "-d", "-s", session)
	tmuxRun("new-window", "-d", "-t", SessionTarget(session), "-n", window, "sleep 600")
	pid := strings.TrimSpace(tmuxRun("list-panes", "-t", WindowTarget(session, window), "-F", "#{pane_pid}"))
	if pid == "" {
		t.Fatal("sleep pane has no pid")
	}

	if _, err := client.ListPanes(ctx, current, window); err == nil || !strings.Contains(err.Error(), "can't find window: "+window) {
		t.Fatalf("wrong-session lookup = %v; want the reported can't find window error", err)
	}
	if exists, err := client.HasWindow(ctx, current, window); err != nil || exists {
		t.Fatalf("HasWindow(current) = %v, %v; want false", exists, err)
	}
	sessions, err := client.FindWindowSessions(ctx, window)
	if err != nil || strings.Join(sessions, ",") != session {
		t.Fatalf("FindWindowSessions = %v, %v; want [%s]", sessions, err, session)
	}
	panes, err := client.ListPanes(ctx, session, window)
	if err != nil || len(panes) != 1 || panes[0].PID != pid || panes[0].Command != "sleep" {
		t.Fatalf("ListPanes(owner) = %#v, %v", panes, err)
	}
	if err := client.StopWindow(ctx, session, window, 2*time.Second); err != nil {
		t.Fatalf("stop window: %v", err)
	}
	if exists, err := client.HasWindow(ctx, session, window); err != nil || exists {
		t.Fatalf("window still exists after stop: %v, %v", exists, err)
	}
	if err := exec.Command("kill", "-0", pid).Run(); err == nil {
		t.Fatalf("sleep process %s still running after stop", pid)
	}
	if exists, err := client.HasWindow(ctx, session+"_extra", window); err != nil || exists {
		t.Fatalf("prefix session matched: %v, %v", exists, err)
	}
}
