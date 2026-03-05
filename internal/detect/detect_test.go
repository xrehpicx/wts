package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestNodeDetector(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]any{
		"name": "test-app",
		"scripts": map[string]string{
			"dev":   "next dev",
			"build": "next build",
			"test":  "vitest",
		},
	}
	data, _ := json.Marshal(pkg)
	writeFile(t, filepath.Join(dir, "package.json"), data)

	d := &NodeDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Type != "nodejs" {
		t.Fatalf("expected type nodejs, got %s", result.Type)
	}
	if len(result.Processes) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(result.Processes))
	}
}

func TestNodeDetector_DetectsRunner(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]any{
		"scripts": map[string]string{"dev": "next dev"},
	}
	data, _ := json.Marshal(pkg)
	writeFile(t, filepath.Join(dir, "package.json"), data)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), []byte(""))

	d := &NodeDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Processes[0].Command != "pnpm run dev" {
		t.Fatalf("expected pnpm runner, got %q", result.Processes[0].Command)
	}
}

func TestNodeDetector_NoPackageJSON(t *testing.T) {
	d := &NodeDetector{}
	result, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for dir without package.json")
	}
}

func TestGoDetector(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22"))
	mkdirAll(t, filepath.Join(dir, "cmd", "api"))
	mkdirAll(t, filepath.Join(dir, "cmd", "worker"))

	d := &GoDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Type != "go" {
		t.Fatalf("expected type go, got %s", result.Type)
	}
	if len(result.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(result.Processes))
	}
}

func TestGoDetector_NoCmdDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22"))

	d := &GoDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Processes) != 1 || result.Processes[0].Name != "run" {
		t.Fatalf("expected fallback 'run' process, got %v", result.Processes)
	}
}

func TestPythonDetector_Django(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), []byte("django\n"))
	writeFile(t, filepath.Join(dir, "manage.py"), []byte("#!/usr/bin/env python\n"))

	d := &PythonDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Type != "python-django" {
		t.Fatalf("expected type python-django, got %s", result.Type)
	}
}

func TestMakefileDetector(t *testing.T) {
	dir := t.TempDir()
	content := `.PHONY: build test lint

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
`
	writeFile(t, filepath.Join(dir, "Makefile"), []byte(content))

	d := &MakefileDetector{}
	result, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	names := make(map[string]bool)
	for _, p := range result.Processes {
		names[p.Name] = true
	}
	if !names["build"] || !names["test"] || !names["lint"] {
		t.Fatalf("expected build, test, lint; got %v", names)
	}
	if names["clean"] {
		t.Fatal("clean should be excluded")
	}
}

func TestCustomDetector(t *testing.T) {
	configDir := t.TempDir()
	detectorDir := filepath.Join(configDir, "detectors")
	mkdirAll(t, detectorDir)

	spec := `name: rust
description: Rust projects
match:
  files:
    - Cargo.toml
processes:
  - name: run
    command: cargo run
  - name: test
    command: cargo test
`
	writeFile(t, filepath.Join(detectorDir, "rust.yaml"), []byte(spec))

	detectors, err := LoadCustomDetectors(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detectors) != 1 {
		t.Fatalf("expected 1 custom detector, got %d", len(detectors))
	}
	if detectors[0].Name() != "rust" {
		t.Fatalf("expected name rust, got %s", detectors[0].Name())
	}

	// Positive detection
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "Cargo.toml"), []byte("[package]\nname = \"test\""))

	result, err := detectors[0].Detect(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(result.Processes))
	}

	// Negative detection
	result, err = detectors[0].Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for dir without Cargo.toml")
	}
}

func TestRun_FirstMatchWins(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]any{
		"scripts": map[string]string{"dev": "next dev"},
	}
	data, _ := json.Marshal(pkg)
	writeFile(t, filepath.Join(dir, "package.json"), data)
	writeFile(t, filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22"))

	result, err := Run(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Type != "nodejs" {
		t.Fatalf("expected nodejs (first match), got %s", result.Type)
	}
}

func TestRun_NoMatch(t *testing.T) {
	result, err := Run(t.TempDir(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for empty dir")
	}
}
