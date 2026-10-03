package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	wtsruntime "github.com/xrehpicx/wts/internal/runtime"
	"github.com/xrehpicx/wts/internal/tmux"
)

func TestCLIManagesProcessesAcrossRealGitWorktrees(t *testing.T) {
	if os.Getenv("WTS_E2E") != "1" {
		t.Skip("set WTS_E2E=1 to run the real git/tmux integration test")
	}
	for _, dependency := range []string{"git", "go", "tmux"} {
		if _, err := exec.LookPath(dependency); err != nil {
			t.Fatalf("%s is required: %v", dependency, err)
		}
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	temp := t.TempDir()
	projectDir := filepath.Join(temp, "project.with.dot")
	featureDir := filepath.Join(temp, "project.with.dot-feature with spaces")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	run(t, projectDir, "git", "init", "-b", "main")
	run(t, projectDir, "git", "config", "user.name", "wts e2e")
	run(t, projectDir, "git", "config", "user.email", "wts-e2e@example.invalid")
	run(t, projectDir, "go", "mod", "init", "example.com/wts-e2e")
	writeFile(t, filepath.Join(projectDir, "main.go"), `package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	name := "default"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	for tick := 1; ; tick++ {
		fmt.Printf("%s tick %d\n", name, tick)
		time.Sleep(100 * time.Millisecond)
	}
}
`)
	run(t, projectDir, "git", "add", "main.go", "go.mod")
	run(t, projectDir, "git", "commit", "-m", "test: add heartbeat app")
	run(t, projectDir, "git", "worktree", "add", "-b", "feature", featureDir)

	configPath := filepath.Join(projectDir, ".wts.yaml")
	writeFile(t, configPath, `version: 1
defaults:
  stop_timeout_sec: 2
  shell: /bin/sh
processes:
  - name: run
    command: 'while true; do echo "run tick"; sleep 0.1; done'
  - name: heartbeat
    command: 'while true; do echo "heartbeat tick"; sleep 0.1; done'
groups:
  - name: dev
    processes: [run, heartbeat]
`)

	binary := filepath.Join(temp, "wts")
	run(t, repoRoot, "go", "build", "-o", binary, ".")
	initOutput := run(t, projectDir, binary, "init", "--dir", projectDir, "--dry-run")
	if !strings.Contains(initOutput, "Detected go project") || !strings.Contains(initOutput, "command: go run .") {
		t.Fatalf("unexpected init output:\n%s", initOutput)
	}
	gitRoot := strings.TrimSpace(run(t, projectDir, "git", "rev-parse", "--show-toplevel"))
	session := tmux.SessionName(gitRoot)
	t.Cleanup(func() {
		cleanupRun(projectDir, binary, "--config", configPath, "stop", "--all")
		cleanupRun(projectDir, "tmux", "kill-session", "-t", session)
	})

	run(t, projectDir, binary, "--config", configPath, "validate")
	run(t, projectDir, binary, "--config", configPath, "start", filepath.Base(projectDir), "--group", "dev")
	waitForStatus(t, projectDir, binary, configPath, func(rows []wtsruntime.StatusRow) bool {
		return len(rows) == 2 && rows[0].Running && len(rows[0].Processes) == 2
	})
	waitForLogs(t, projectDir, binary, configPath, filepath.Base(projectDir), "run", "run tick")

	run(t, projectDir, binary, "--config", configPath, "restart", filepath.Base(projectDir), "--process", "run")
	waitForStatus(t, projectDir, binary, configPath, func(rows []wtsruntime.StatusRow) bool {
		return len(rows) == 2 && rows[0].Running && len(rows[0].Processes) == 2
	})
	run(t, projectDir, binary, "--config", configPath, "switch", filepath.Base(featureDir), "--process", "run")
	waitForStatus(t, projectDir, binary, configPath, func(rows []wtsruntime.StatusRow) bool {
		return len(rows) == 2 && !rows[0].Running && rows[1].Running && rows[1].Active
	})

	run(t, projectDir, binary, "--config", configPath, "stop", "--all")
	waitForStatus(t, projectDir, binary, configPath, func(rows []wtsruntime.StatusRow) bool {
		return len(rows) == 2 && !rows[0].Running && !rows[1].Running
	})
}

// TestCLIStopsProcessesStartedFromAnotherWorktree covers the 2026-10-03 bug:
// a group started from inside a linked worktree was invisible to wts run from
// the main worktree, so stop reported success while the processes kept
// running and status showed the worktree stopped.
func TestCLIStopsProcessesStartedFromAnotherWorktree(t *testing.T) {
	if os.Getenv("WTS_E2E") != "1" {
		t.Skip("set WTS_E2E=1 to run the real git/tmux integration test")
	}
	for _, dependency := range []string{"git", "go", "tmux"} {
		if _, err := exec.LookPath(dependency); err != nil {
			t.Fatalf("%s is required: %v", dependency, err)
		}
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	temp := t.TempDir()
	projectDir := filepath.Join(temp, "imai")
	featureDir := filepath.Join(temp, "t3code-seed-e2e-members-badge")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, projectDir, "git", "init", "-b", "main")
	run(t, projectDir, "git", "-c", "user.name=wts e2e", "-c", "user.email=wts-e2e@example.invalid", "commit", "--allow-empty", "-m", "init")
	run(t, projectDir, "git", "worktree", "add", "-b", "feature", featureDir)
	config := `version: 1
defaults:
  stop_timeout_sec: 2
  shell: /bin/sh
processes:
  - name: auth:generate
    command: 'echo generated'
  - name: dev
    command: 'while true; do echo "dev tick"; sleep 0.1; done'
  - name: convex:dev
    command: 'while true; do echo "convex tick"; sleep 0.1; done'
groups:
  - name: dev servers w/o trigger
    processes: [dev, convex:dev]
`
	// Each worktree carries its own checked-in config, as in the bug report.
	mainConfig := filepath.Join(projectDir, ".wts.yaml")
	featureConfig := filepath.Join(featureDir, ".wts.yaml")
	writeFile(t, mainConfig, config)
	writeFile(t, featureConfig, config)

	binary := filepath.Join(temp, "wts")
	run(t, repoRoot, "go", "build", "-o", binary, ".")
	mainRoot := strings.TrimSpace(run(t, projectDir, "git", "rev-parse", "--show-toplevel"))
	featureRoot := strings.TrimSpace(run(t, featureDir, "git", "rev-parse", "--show-toplevel"))
	// Selectors use git's symlink-free paths (macOS TempDir lives under /var).
	legacySession := tmux.SessionName(featureRoot)
	t.Cleanup(func() {
		cleanupRun(projectDir, binary, "--config", mainConfig, "stop", "--all")
		cleanupRun(projectDir, "tmux", "kill-session", "-t", tmux.SessionTarget(tmux.SessionName(mainRoot)))
		cleanupRun(projectDir, "tmux", "kill-session", "-t", tmux.SessionTarget(legacySession))
	})

	statusFromMain := func() wtsruntime.StatusRow {
		var row wtsruntime.StatusRow
		output := run(t, projectDir, binary, "--config", mainConfig, "status", featureRoot, "--json")
		if err := json.Unmarshal([]byte(output), &row); err != nil {
			t.Fatalf("decode status: %v\n%s", err, output)
		}
		return row
	}

	// Started from the feature worktree, inspected and stopped from main.
	run(t, featureDir, binary, "--config", featureConfig, "start", featureRoot, "--group", "dev servers w/o trigger")
	waitForStatus(t, projectDir, binary, mainConfig, func(rows []wtsruntime.StatusRow) bool {
		for _, row := range rows {
			if row.Dir == featureRoot {
				return row.Running && len(row.Processes) == 2
			}
		}
		return false
	})
	if output := run(t, projectDir, binary, "--config", mainConfig, "stop", filepath.Base(featureDir), "--process", "dev"); !strings.Contains(output, "stopped dev") {
		t.Fatalf("unexpected stop --process output: %s", output)
	}
	if row := statusFromMain(); len(row.Processes) != 1 || row.Processes[0].Name != "convex:dev" {
		t.Fatalf("after stopping dev: %#v", row)
	}
	if output := run(t, projectDir, binary, "--config", mainConfig, "stop", featureRoot); !strings.Contains(output, "stopped all processes") {
		t.Fatalf("unexpected stop output: %s", output)
	}
	if row := statusFromMain(); row.Running {
		t.Fatalf("feature still running after stop: %#v", row)
	}
	if output := run(t, projectDir, binary, "--config", mainConfig, "stop", featureRoot); !strings.Contains(output, "nothing running") {
		t.Fatalf("second stop should report nothing running: %s", output)
	}

	// A window left by wts 0.4.0 in a session named after the feature
	// worktree, while a different session exists.
	window := tmux.WindowName(featureRoot)
	run(t, projectDir, "tmux", "new-session", "-d", "-s", legacySession)
	run(t, projectDir, "tmux", "new-window", "-d", "-t", tmux.SessionTarget(legacySession), "-n", window, "sleep 600")
	if row := statusFromMain(); !row.Running {
		t.Fatalf("status from main missed the legacy window: %#v", row)
	}
	if output := run(t, projectDir, binary, "--config", mainConfig, "stop", featureRoot); !strings.Contains(output, "stopped all processes") {
		t.Fatalf("unexpected legacy stop output: %s", output)
	}
	windows := run(t, projectDir, "tmux", "list-windows", "-t", tmux.SessionTarget(legacySession), "-F", "#{window_name}")
	if strings.Contains(windows, window) {
		t.Fatalf("legacy window survived stop: %s", windows)
	}
}

func waitForLogs(t *testing.T, dir, binary, configPath, worktree, process, expected string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		output = run(t, dir, binary, "--config", configPath, "logs", worktree, "--process", process, "--lines", "20")
		if strings.Contains(output, expected) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("expected %q in process logs, got:\n%s", expected, output)
}

func waitForStatus(t *testing.T, dir, binary, configPath string, ready func([]wtsruntime.StatusRow) bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last []wtsruntime.StatusRow
	for time.Now().Before(deadline) {
		output := run(t, dir, binary, "--config", configPath, "status", "--json")
		if err := json.Unmarshal([]byte(output), &last); err != nil {
			t.Fatalf("decode status: %v\n%s", err, output)
		}
		if ready(last) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("status did not converge: %#v", last)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func cleanupRun(dir, name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	_ = cmd.Run()
}

func Example_e2eOptIn() {
	fmt.Println("WTS_E2E=1 go test -count=1 ./integration")
	// Output: WTS_E2E=1 go test -count=1 ./integration
}
