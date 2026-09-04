package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrehpicx/wts/internal/config"
)

func TestInitProtectsExistingConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		flags            []string
		wantErr, replace bool
	}{
		{name: "existing config", wantErr: true},
		{name: "dry run", flags: []string{"--dry-run"}},
		{name: "force", flags: []string{"--force"}, replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, config.DefaultConfigFile)
			original := []byte("# preserve my config\n")
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := NewRootCmd("dev", "")
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs(append([]string{"init", "--dir", dir}, tc.flags...))
			err := cmd.Execute()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Execute() error = %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.replace {
				if _, err := config.Load(path); err != nil {
					t.Fatalf("generated config is invalid: %v", err)
				}
			} else if !bytes.Equal(got, original) {
				t.Fatalf("existing file changed: %q", got)
			}
			if len(tc.flags) > 0 && !strings.Contains(output.String(), "Detected go project") {
				t.Fatalf("missing detection output: %s", output.String())
			}
		})
	}
}
