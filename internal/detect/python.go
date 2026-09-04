package detect

import (
	"fmt"
	"path/filepath"
)

// PythonDetector recognizes Python projects by common markers (pyproject.toml,
// requirements.txt, setup.py, manage.py).
type PythonDetector struct{}

func (d *PythonDetector) Name() string { return "python" }

func (d *PythonDetector) Detect(dir string) (*Result, error) {
	markers := []string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg", "manage.py"}
	found := false
	for _, m := range markers {
		exists, err := regularFileExists(filepath.Join(dir, m))
		if err != nil {
			return nil, fmt.Errorf("check %s: %w", m, err)
		}
		if exists {
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}

	var procs []Process
	prefix, err := pythonCommandPrefix(dir)
	if err != nil {
		return nil, err
	}

	manageExists, err := regularFileExists(filepath.Join(dir, "manage.py"))
	if err != nil {
		return nil, fmt.Errorf("check manage.py: %w", err)
	}
	if manageExists {
		procs = append(procs, Process{
			Name:    "runserver",
			Command: prefix + "python manage.py runserver",
		})
		procs = append(procs, Process{
			Name:    "test",
			Command: prefix + "python manage.py test",
		})
		return &Result{Type: "python-django", Processes: procs}, nil
	}

	runCmd, err := pythonRunCommand(dir, prefix)
	if err != nil {
		return nil, err
	}
	if runCmd != "" {
		procs = append(procs, Process{
			Name:    "run",
			Command: runCmd,
		})
	}
	if testCmd := pythonTestCommand(prefix); testCmd != "" {
		procs = append(procs, Process{
			Name:    "test",
			Command: testCmd,
		})
	}

	if len(procs) == 0 {
		return nil, nil
	}
	return &Result{Type: "python", Processes: procs}, nil
}

func pythonCommandPrefix(dir string) (string, error) {
	for _, candidate := range []struct {
		file   string
		prefix string
	}{
		{file: "poetry.lock", prefix: "poetry run "},
		{file: "uv.lock", prefix: "uv run "},
	} {
		exists, err := regularFileExists(filepath.Join(dir, candidate.file))
		if err != nil {
			return "", fmt.Errorf("check %s: %w", candidate.file, err)
		}
		if exists {
			return candidate.prefix, nil
		}
	}
	return "", nil
}

func pythonRunCommand(dir, prefix string) (string, error) {
	for _, entry := range []string{"main.py", "app.py"} {
		exists, err := regularFileExists(filepath.Join(dir, entry))
		if err != nil {
			return "", fmt.Errorf("check %s: %w", entry, err)
		}
		if exists {
			return prefix + "python " + entry, nil
		}
	}
	return "", nil
}

func pythonTestCommand(prefix string) string {
	if prefix == "" {
		return ""
	}
	return prefix + "pytest"
}
