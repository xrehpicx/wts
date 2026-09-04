package model

import "testing"

func TestResolveTargetDefaultsToFirstProcess(t *testing.T) {
	t.Parallel()

	project := NewProject("/tmp/.wts.yaml", "/tmp", Config{
		Version: CurrentVersion,
		Processes: []Process{
			{Name: "api", Command: "go run ."},
			{Name: "web", Command: "pnpm dev"},
		},
	})

	target, err := project.ResolveTarget("", "")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.Kind != TargetProcess || target.Name != "api" {
		t.Fatalf("unexpected default target: %#v", target)
	}
}

func TestConfigReturnsDeepCopy(t *testing.T) {
	t.Parallel()

	project := NewProject("/tmp/.wts.yaml", "/tmp", Config{
		Version: CurrentVersion,
		Processes: []Process{{
			Name:    "api",
			Command: "go run .",
			Env:     map[string]string{"PORT": "8080"},
		}},
		Groups: []ProcessGroup{{Name: "dev", Processes: []string{"api"}}},
	})

	cfg := project.Config()
	cfg.Processes[0].Env["PORT"] = "9090"
	cfg.Groups[0].Processes[0] = "changed"

	if got := project.Processes[0].Env["PORT"]; got != "8080" {
		t.Fatalf("project environment mutated through Config(): %q", got)
	}
	if got := project.Groups[0].Processes[0]; got != "api" {
		t.Fatalf("project group mutated through Config(): %q", got)
	}
}

func TestNewProjectOwnsItsConfiguration(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Processes: []Process{{Name: "api", Command: "go run .", Env: map[string]string{"PORT": "8080"}}},
		Groups:    []ProcessGroup{{Name: "dev", Processes: []string{"api"}}},
	}
	project := NewProject("/tmp/.wts.yaml", "/tmp", cfg)
	cfg.Processes[0].Name = "changed"
	cfg.Processes[0].Env["PORT"] = "9090"
	cfg.Groups[0].Processes[0] = "changed"
	proc, err := project.Process("api")
	if err != nil || proc.Name != "api" || proc.Env["PORT"] != "8080" {
		t.Fatalf("configuration mutation changed indexed process: %#v, %v", proc, err)
	}
	if project.Groups[0].Processes[0] != "api" {
		t.Fatal("configuration mutation changed group membership")
	}
}
