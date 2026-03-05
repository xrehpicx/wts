package cli

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPickerFallsBackWhenFZFUnavailable(t *testing.T) {
	t.Parallel()

	in := bytes.NewBufferString("2\n")
	out := &bytes.Buffer{}
	picker := &Picker{
		Input:  in,
		Output: out,
		ErrOut: &bytes.Buffer{},
		LookPath: func(string) (string, error) {
			return "", fmt.Errorf("not found")
		},
		RunFZF: nil,
	}

	selected, err := picker.Select([]string{"api", "web"})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selected != "web" {
		t.Fatalf("unexpected selected workspace: %q", selected)
	}
}
