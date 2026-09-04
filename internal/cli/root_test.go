package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xrehpicx/wts/internal/gitwt"
)

func TestRunOptionsFromFlagsRejectsMixedProcessAndGroup(t *testing.T) {
	t.Parallel()

	if _, err := runOptionsFromFlags("api", "dev", false); err == nil {
		t.Fatal("expected error when both process and group are set")
	}
}

func TestValidateStopSelectionRequiresWorktreeForTarget(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		process string
		group   string
	}{
		{name: "process", process: "api"},
		{name: "group", group: "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateStopSelection(false, tc.process, tc.group, nil)
			if err == nil || !strings.Contains(err.Error(), "worktree") {
				t.Fatalf("expected worktree requirement error, got %v", err)
			}
		})
	}
}

func TestNextWorktreeIndexStartsAtEdgeWhenNothingIsActive(t *testing.T) {
	t.Parallel()

	items := []gitwt.Worktree{{Dir: "/tmp/a"}, {Dir: "/tmp/b"}, {Dir: "/tmp/c"}}
	if got := nextWorktreeIndex(items, "", 1); got != 0 {
		t.Fatalf("next index with no active worktree = %d; want 0", got)
	}
	if got := nextWorktreeIndex(items, "", -1); got != 2 {
		t.Fatalf("previous index with no active worktree = %d; want 2", got)
	}
}

func TestRootCommandRunsTUIByDefault(t *testing.T) {
	t.Parallel()

	called := false
	a := &app{
		runTUI: func(context.Context) error {
			called = true
			return nil
		},
	}

	cmd := a.newRootCmd()
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute root command: %v", err)
	}
	if !called {
		t.Fatal("expected bare root command to launch TUI")
	}
}

func TestRootUsesCommandOutput(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	cmd := NewRootCmd("1.2.3", "abc1234")
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "wts 1.2.3 (abc1234)\n" {
		t.Fatalf("version output = %q", got)
	}
}

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRootPropagatesOutputFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("output closed")
	cmd := NewRootCmd("dev", "")
	cmd.SetOut(failedWriter{failure})
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); !errors.Is(err, failure) {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestRootRejectsUnknownArguments(t *testing.T) {
	t.Parallel()
	called := false
	a := &app{runTUI: func(context.Context) error { called = true; return nil }}
	cmd := a.newRootCmd()
	cmd.SetArgs([]string{"typo"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected unknown command error")
	}
	if called {
		t.Fatal("unexpected TUI launch")
	}
}

func TestTUICommandReceivesContext(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"tui"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			a := &app{runTUI: func(got context.Context) error {
				if got != ctx {
					t.Error("command context was not forwarded")
				}
				return got.Err()
			}}
			cmd := a.newRootCmd()
			cmd.SetArgs(args)
			if err := cmd.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("ExecuteContext() error = %v", err)
			}
		})
	}
}

func TestResolveRepoRootPreservesWhitespace(t *testing.T) {
	t.Parallel()
	name := "repo with space"
	if runtime.GOOS != "windows" {
		name = "repo with tab\tand newline\n "
	}
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", dir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	got, err := resolveRepoRoot(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// Git resolves macOS's /var symlink while TempDir may not.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveRepoRoot() = %q; want %q", got, want)
	}
}

func TestNextWorktreeIndexWraps(t *testing.T) {
	t.Parallel()
	items := []gitwt.Worktree{{Dir: "/tmp/a"}, {Dir: "/tmp/b"}, {Dir: "/tmp/c"}}
	for _, tc := range []struct {
		active      string
		delta, want int
	}{
		{"/tmp/c", 1, 0}, {"/tmp/a", -1, 2}, {"/tmp/b", -8, 2}, {"/tmp/b", 8, 0},
	} {
		if got := nextWorktreeIndex(items, tc.active, tc.delta); got != tc.want {
			t.Errorf("nextWorktreeIndex(%q,%d) = %d; want %d", tc.active, tc.delta, got, tc.want)
		}
	}
}

func TestWorktreeLabelEscapesControlCharacters(t *testing.T) {
	t.Parallel()
	wt := gitwt.Worktree{Name: "evil\x1b[2J", Branch: "feature", Dir: "/tmp/line\nname\t"}
	label := worktreeLabel(wt)
	if strings.ContainsAny(label, "\n\t\x1b") {
		t.Fatalf("label contains terminal controls: %q", label)
	}
	if !strings.Contains(label, `\n`) {
		t.Fatalf("escaped newline missing: %q", label)
	}
	if displayField("\n") == displayField(`"\n"`) {
		t.Fatal("literal and escaped fields collide")
	}
}
