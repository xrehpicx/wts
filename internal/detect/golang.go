package detect

import (
	"os"
	"path/filepath"
)

// GoDetector recognizes Go projects by go.mod and infers processes from cmd/
// sub-directories.
type GoDetector struct{}

func (d *GoDetector) Name() string { return "go" }

func (d *GoDetector) Detect(dir string) (*Result, error) {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return nil, nil
	}

	var procs []Process
	cmdDir := filepath.Join(dir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				procs = append(procs, Process{
					Name:    e.Name(),
					Command: "go run ./cmd/" + e.Name(),
				})
			}
		}
	}

	if len(procs) == 0 {
		procs = append(procs, Process{
			Name:    "run",
			Command: "go run .",
		})
	}

	return &Result{Type: "go", Processes: procs}, nil
}
