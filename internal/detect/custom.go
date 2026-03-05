package detect

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// customDetectorSpec is the on-disk YAML format for user-defined detectors
// stored in <configDir>/detectors/*.yaml.
//
// Example:
//
//	name: rust
//	description: Rust projects with Cargo
//	match:
//	  files:
//	    - Cargo.toml
//	processes:
//	  - name: run
//	    command: cargo run
//	  - name: test
//	    command: cargo watch -x test
type customDetectorSpec struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Match       struct {
		Files []string `yaml:"files"`
	} `yaml:"match"`
	Processes []struct {
		Name    string `yaml:"name"`
		Command string `yaml:"command"`
	} `yaml:"processes"`
}

// CustomDetector is a Detector backed by a YAML spec file.
type CustomDetector struct {
	spec customDetectorSpec
}

func (d *CustomDetector) Name() string { return d.spec.Name }

func (d *CustomDetector) Detect(dir string) (*Result, error) {
	for _, f := range d.spec.Match.Files {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return nil, nil
		}
	}

	procs := make([]Process, 0, len(d.spec.Processes))
	for _, p := range d.spec.Processes {
		procs = append(procs, Process{Name: p.Name, Command: p.Command})
	}
	if len(procs) == 0 {
		return nil, nil
	}
	return &Result{Type: d.spec.Name, Processes: procs}, nil
}

// LoadCustomDetectors reads every .yaml / .yml file under dir/detectors/ and
// returns them as Detector instances. Errors on individual files are silently
// skipped so a single bad file doesn't break all detection.
func LoadCustomDetectors(configDir string) ([]Detector, error) {
	dir := filepath.Join(configDir, "detectors")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var detectors []Detector
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var spec customDetectorSpec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			continue
		}
		if spec.Name == "" || len(spec.Match.Files) == 0 || len(spec.Processes) == 0 {
			continue
		}
		detectors = append(detectors, &CustomDetector{spec: spec})
	}
	return detectors, nil
}

// ConfigDir returns the default config directory path for wts
// (~/.config/wts on Unix).
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "wts")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "wts")
}
