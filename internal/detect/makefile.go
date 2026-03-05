package detect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var makeTargetRe = regexp.MustCompile(`^([a-zA-Z0-9._-]+)\s*:`)

// Targets that are almost always infrastructure, not user-runnable processes.
var makeSkipTargets = map[string]bool{
	"all": true, "clean": true, "install": true, "uninstall": true,
	"dist": true, "distclean": true, "check": true, "help": true,
	".PHONY": true, ".DEFAULT": true,
}

// MakefileDetector recognizes projects with a Makefile and extracts targets as
// processes. This is a low-priority fallback detector.
type MakefileDetector struct{}

func (d *MakefileDetector) Name() string { return "makefile" }

func (d *MakefileDetector) Detect(dir string) (*Result, error) {
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if procs := parseMakefile(filepath.Join(dir, name)); len(procs) > 0 {
			return &Result{Type: "makefile", Processes: procs}, nil
		}
	}
	return nil, nil
}

func parseMakefile(path string) []Process {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var procs []Process
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		matches := makeTargetRe.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		target := matches[1]
		if makeSkipTargets[target] || strings.HasPrefix(target, ".") {
			continue
		}
		procs = append(procs, Process{
			Name:    target,
			Command: "make " + target,
		})
	}
	return procs
}
