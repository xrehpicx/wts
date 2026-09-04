package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestPickerFallbackInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, want string
		wantErr           bool
	}{
		{name: "EOF terminated input", input: "2", want: "second"},
		{name: "surrounding whitespace", input: " 1 \n", want: "first"},
		{name: "empty input", wantErr: true},
		{name: "non numeric", input: "first\n", wantErr: true},
		{name: "zero", input: "0\n", wantErr: true},
		{name: "negative", input: "-1\n", wantErr: true},
		{name: "too large", input: "3\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			picker := &Picker{Input: strings.NewReader(tc.input), Output: io.Discard}
			got, err := picker.Select([]string{"first", "second"})
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("Select() = %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestPickerDoesNotFallBackAfterFZF(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, selection string
		failure         error
		wantErr         bool
	}{
		{name: "valid", selection: "second"},
		{name: "unknown", selection: "not a worktree", wantErr: true},
		{name: "empty", wantErr: true},
		{name: "canceled", failure: ErrSelectionCanceled, wantErr: true},
		{name: "failed", failure: errors.New("fzf failed"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			picker := &Picker{
				Input: strings.NewReader("1\n"), Output: &out,
				LookPath: func(string) (string, error) { return "fzf", nil },
				RunFZF:   func(context.Context, string, []string, io.Writer) (string, error) { return tc.selection, tc.failure },
			}
			got, err := picker.Select([]string{"first", "second"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Select() = %q, %v", got, err)
			}
			if tc.failure != nil && !errors.Is(err, tc.failure) {
				t.Fatalf("error lost: %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected fallback prompt: %s", out.String())
			}
		})
	}
}

func TestRunFZFPreservesRecordWhitespace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell shim")
	}
	// This shim echoes stdin to verify record framing without requiring fzf.
	path := filepath.Join(t.TempDir(), "fzf")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	name := " leading\tname\ntrailing "
	got, err := runFZF(context.Background(), path, []string{name}, io.Discard)
	if err != nil || got != name {
		t.Fatalf("runFZF() = %q, %v; want %q", got, err, name)
	}
}

func TestRunFZFCancel(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell shim")
	}
	path := filepath.Join(t.TempDir(), "fzf")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 130\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := runFZF(context.Background(), path, []string{"name"}, io.Discard)
	if !errors.Is(err, ErrSelectionCanceled) {
		t.Fatalf("runFZF() error = %v", err)
	}
}

func TestPickerCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	picker := &Picker{Input: strings.NewReader("1\n"), Output: io.Discard}
	_, err := picker.SelectContext(ctx, []string{"one"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SelectContext() error = %v", err)
	}
}
