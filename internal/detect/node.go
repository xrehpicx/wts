package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// NodeDetector recognizes Node.js / TypeScript projects by the presence of a
// package.json and extracts npm scripts as processes.
type NodeDetector struct{}

func (d *NodeDetector) Name() string { return "nodejs" }

func (d *NodeDetector) Detect(dir string) (*Result, error) {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	if len(pkg.Scripts) == 0 {
		return nil, nil
	}

	runner := npmRunner(dir)

	var procs []Process
	for name := range pkg.Scripts {
		procs = append(procs, Process{
			Name:    name,
			Command: runner + " run " + name,
		})
	}
	return &Result{Type: "nodejs", Processes: procs}, nil
}

// npmRunner returns "pnpm", "yarn", or "npm" based on lock file presence.
func npmRunner(dir string) string {
	for _, pair := range []struct {
		lock   string
		runner string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
	} {
		if _, err := os.Stat(filepath.Join(dir, pair.lock)); err == nil {
			return pair.runner
		}
	}
	return "npm"
}
