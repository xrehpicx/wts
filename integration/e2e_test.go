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
