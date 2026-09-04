package tmux

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, ...string) (string, error)

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) (string, error) {
	return f(ctx, name, args...)
}

func TestParsePaneListPreservesEmptyProcessField(t *testing.T) {
	t.Parallel()

	panes := parsePaneList("%0\t\tbash\t123\tbash\t0\n%1\tapi\twts:api\t456\tgo\t1")
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	if panes[0].ID != "%0" || panes[0].Process != "" || panes[0].Title != "bash" || panes[0].PID != "123" {
		t.Fatalf("empty process shifted pane fields: %#v", panes[0])
	}
	if panes[1].Process != "api" || !panes[1].Dead {
		t.Fatalf("unexpected managed pane: %#v", panes[1])
	}
}

func TestBuildPayloadSortsAndQuotesEnvironment(t *testing.T) {
	t.Parallel()

	got := buildPayload("go run .", map[string]string{
		"Z_LAST":  "plain",
		"A_FIRST": "it's safe",
	})
	want := `export A_FIRST='it'"'"'s safe'; export Z_LAST='plain'; go run .`
	if got != want {
		t.Fatalf("buildPayload() = %q; want %q", got, want)
	}
}

func TestHasWindowDoesNotHideUnexpectedTmuxErrors(t *testing.T) {
	t.Parallel()

	client := NewClient("tmux")
	client.runner = runnerFunc(func(context.Context, string, ...string) (string, error) {
		return "", errors.New("permission denied")
	})

	exists, err := client.HasWindow(context.Background(), "session", "window")
	if err == nil || exists {
		t.Fatalf("expected unexpected tmux error, got exists=%v err=%v", exists, err)
	}
}

func TestMissingLinuxTmuxSocketIsTreatedAsNoServer(t *testing.T) {
	t.Parallel()

	err := errors.New("error connecting to /tmp/tmux-1001/default (No such file or directory)")
	if !isMissingTmuxTarget(err) {
		t.Fatal("expected absent tmux socket to be treated as a missing server")
	}
}

func TestEnsureSessionCreatesOnlyWhenSessionIsMissing(t *testing.T) {
	t.Parallel()

	for _, message := range []string{
		"can't find session: session",
		"no such session: session",
	} {
		message := message
		t.Run(message, func(t *testing.T) {
			t.Parallel()

			var calls []string
			client := NewClient("tmux")
			client.runner = runnerFunc(func(_ context.Context, _ string, args ...string) (string, error) {
				calls = append(calls, strings.Join(args, " "))
				if args[0] == "has-session" {
					return "", errors.New(message)
				}
				return "", nil
			})

			if err := client.EnsureSession(context.Background(), "session"); err != nil {
				t.Fatalf("ensure session: %v", err)
			}
			if len(calls) != 2 || !strings.HasPrefix(calls[1], "new-session") {
				t.Fatalf("unexpected tmux calls: %#v", calls)
			}
		})
	}
}

func TestGetSessionOptionDoesNotHideUnexpectedErrors(t *testing.T) {
	t.Parallel()

	client := NewClient("tmux")
	client.runner = runnerFunc(func(context.Context, string, ...string) (string, error) {
		return "", errors.New("server protocol error")
	})

	if _, err := client.GetSessionOption(context.Background(), "session", "@option"); err == nil {
		t.Fatal("expected option read error")
	}
}

func TestStartWindowCleansUpWhenPaneHardeningFails(t *testing.T) {
	t.Parallel()

	var killed bool
	client := NewClient("tmux")
	client.runner = runnerFunc(func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "list-windows":
			return "", errors.New("can't find window: window")
		case "new-window":
			return "", nil
		case "select-pane":
			return "", errors.New("cannot set title")
		case "kill-window":
			killed = true
			return "", nil
		default:
			return "", nil
		}
	})

	err := client.StartWindowCommand(
		context.Background(), "session", "window", "/tmp", "/bin/sh", "echo ok", nil, ProcessPaneTitle("dev"),
	)
	if err == nil || !killed {
		t.Fatalf("expected hardening error and cleanup, got err=%v killed=%v", err, killed)
	}
}

func TestStopWindowClosesAsSoonAsManagedProcessesExit(t *testing.T) {
	t.Parallel()

	var killed bool
	client := NewClient("tmux")
	client.runner = runnerFunc(func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "list-windows", "send-keys":
			return "", nil
		case "list-panes":
			return "%0\tapi\twts:api\t123\tsh\t0", nil
		case "kill-window":
			killed = true
			return "", nil
		default:
			return "", nil
		}
	})

	started := time.Now()
	if err := client.StopWindow(context.Background(), "session", "window", 2*time.Second); err != nil {
		t.Fatalf("stop window: %v", err)
	}
	if !killed {
		t.Fatal("expected exited window to be removed")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("stop waited for timeout after process exit: %v", elapsed)
	}
}

func TestStopWindowInterruptsEveryLivePane(t *testing.T) {
	t.Parallel()

	var interrupted []string
	client := NewClient("tmux")
	client.runner = runnerFunc(func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "list-windows", "kill-window":
			return "", nil
		case "list-panes":
			command := "go"
			if len(interrupted) == 2 {
				command = "sh"
			}
			return "%1\tapi\twts:api\t123\t" + command + "\t0\n" +
				"%2\tweb\twts:web\t456\t" + command + "\t0", nil
		case "send-keys":
			interrupted = append(interrupted, args[2])
			return "", nil
		default:
			return "", nil
		}
	})

	if err := client.StopWindow(context.Background(), "session", "window", time.Second); err != nil {
		t.Fatalf("stop window: %v", err)
	}
	if strings.Join(interrupted, ",") != "%1,%2" {
		t.Fatalf("interrupted panes = %v; want both live panes", interrupted)
	}
}

func TestStopPaneClosesAsSoonAsCommandReturnsToShell(t *testing.T) {
	t.Parallel()

	var interrupted, killed bool
	client := NewClient("tmux")
	client.runner = runnerFunc(func(_ context.Context, _ string, args ...string) (string, error) {
		switch args[0] {
		case "display-message":
			if interrupted {
				return "123\t0\tsh", nil
			}
			return "123\t0\tgo", nil
		case "send-keys":
			interrupted = true
			return "", nil
		case "kill-pane":
			killed = true
			return "", nil
		default:
			return "", nil
		}
	})

	started := time.Now()
	if err := client.StopPane(context.Background(), "%1", 2*time.Second); err != nil {
		t.Fatalf("stop pane: %v", err)
	}
	if !interrupted || !killed {
		t.Fatalf("expected interrupt and cleanup, got interrupted=%v killed=%v", interrupted, killed)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("stop waited for timeout after command exit: %v", elapsed)
	}
}

func TestBuildPayloadPreservesShellPrograms(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`printf first; printf second`,
		`printf first && printf second`,
		`VALUE=first; printf '%s' "$VALUE"; printf second`,
		`for part in first second; do printf '%s' "$part"; done`,
	} {
		t.Run(command, func(t *testing.T) {
			out, err := exec.Command("/bin/sh", "-c", buildPayload(command, nil)).CombinedOutput()
			if err != nil || string(out) != "firstsecond" {
				t.Fatalf("shell program = %q, %v; want firstsecond", out, err)
			}
		})
	}
}

func TestExecRunnerPreservesLogWhitespace(t *testing.T) {
	t.Parallel()
	out, err := (execRunner{}).Run(context.Background(), "/bin/sh", "-c", `printf '  indented\n\n'`)
	if err != nil || out != "  indented\n\n" {
		t.Fatalf("output = %q, %v; want original whitespace", out, err)
	}
}

func TestStopShellCommandWaitsForLiveChildren(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"pane", "window"} {
		t.Run(scope, func(t *testing.T) {
			interrupted, killed := false, false
			client := NewClient("tmux")
			client.runner = runnerFunc(func(_ context.Context, name string, args ...string) (string, error) {
				if name == "pgrep" {
					if !interrupted {
						return "456", nil
					}
					return "", nil
				}
				switch args[0] {
				case "display-message":
					return "123\t0\tsh", nil
				case "list-panes":
					return "%1\tapi\twts:api\t123\tsh\t0", nil
				case "send-keys":
					interrupted = true
				case "kill-pane", "kill-window":
					killed = true
				}
				return "", nil
			})
			var err error
			if scope == "pane" {
				err = client.StopPane(context.Background(), "%1", time.Second)
			} else {
				err = client.StopWindow(context.Background(), "session", "window", time.Second)
			}
			if err != nil || !interrupted || !killed {
				t.Fatalf("stop %s: interrupted=%v killed=%v err=%v", scope, interrupted, killed, err)
			}
		})
	}
}

func TestGetSessionOptionPreservesPathWhitespace(t *testing.T) {
	t.Parallel()
	want := "/tmp/worktree \n"
	client := NewClient("tmux")
	client.runner = runnerFunc(func(context.Context, string, ...string) (string, error) { return want + "\n", nil })
	got, err := client.GetSessionOption(context.Background(), "session", ActiveWorktreeOptionKey())
	if err != nil || got != want {
		t.Fatalf("option = %q, %v; want %q", got, err, want)
	}
}
