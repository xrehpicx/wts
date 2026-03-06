package detect

import (
	"fmt"
	"sort"
)

// Process is a single inferred process entry.
type Process struct {
	Name    string
	Command string
}

// Result is what a Detector returns when it recognizes a project.
type Result struct {
	Type      string
	Processes []Process
}

// Detector inspects a directory and optionally returns detected processes.
// Returning nil, nil means "not my project type, skip me."
type Detector interface {
	Name() string
	Detect(dir string) (*Result, error)
}

var builtins []Detector

func init() {
	builtins = []Detector{
		&NodeDetector{},
		&GoDetector{},
		&PythonDetector{},
		&MakefileDetector{},
	}
}

// All returns every available detector: built-ins followed by custom detectors
// loaded from configDir/detectors/*.yaml. If configDir is empty, only built-ins
// are returned.
func All(configDir string) []Detector {
	all := make([]Detector, len(builtins))
	copy(all, builtins)

	if configDir != "" {
		custom, _ := LoadCustomDetectors(configDir)
		all = append(all, custom...)
	}
	return all
}

// Run tries each detector in order and returns the first match. Custom
// detectors from configDir are appended after built-ins. Returns nil, nil
// if nothing matched.
func Run(dir, configDir string) (*Result, error) {
	for _, d := range All(configDir) {
		result, err := d.Detect(dir)
		if err != nil {
			return nil, fmt.Errorf("%s detector: %w", d.Name(), err)
		}
		if result != nil {
			sort.Slice(result.Processes, func(i, j int) bool {
				return result.Processes[i].Name < result.Processes[j].Name
			})
			return result, nil
		}
	}
	return nil, nil
}
