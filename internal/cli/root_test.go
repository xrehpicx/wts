package cli

import (
	"context"
	"testing"
)

func TestRunOptionsFromFlagsRejectsMixedProcessAndGroup(t *testing.T) {
	t.Parallel()

	if _, err := runOptionsFromFlags("api", "dev", false); err == nil {
		t.Fatal("expected error when both process and group are set")
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
