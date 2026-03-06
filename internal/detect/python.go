package detect

import (
	"os"
	"path/filepath"
)

// PythonDetector recognizes Python projects by common markers (pyproject.toml,
// requirements.txt, setup.py, manage.py).
type PythonDetector struct{}

func (d *PythonDetector) Name() string { return "python" }

func (d *PythonDetector) Detect(dir string) (*Result, error) {
	markers := []string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg"}
	found := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}

	var procs []Process

	if _, err := os.Stat(filepath.Join(dir, "manage.py")); err == nil {
		procs = append(procs, Process{
			Name:    "runserver",
			Command: "python manage.py runserver",
		})
		procs = append(procs, Process{
			Name:    "test",
			Command: "python manage.py test",
		})
		return &Result{Type: "python-django", Processes: procs}, nil
	}

	if runCmd := pythonRunCommand(dir); runCmd != "" {
		procs = append(procs, Process{
			Name:    "run",
			Command: runCmd,
		})
	}
	if testCmd := pythonTestCommand(dir); testCmd != "" {
		procs = append(procs, Process{
			Name:    "test",
			Command: testCmd,
		})
	}

	return &Result{Type: "python", Processes: procs}, nil
}

func pythonCommandPrefix(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "poetry.lock")); err == nil {
		return "poetry run "
	}
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); err == nil {
		return "uv run "
	}
	return ""
}

func pythonRunCommand(dir string) string {
	prefix := pythonCommandPrefix(dir)
	for _, entry := range []string{"main.py", "app.py"} {
		if _, err := os.Stat(filepath.Join(dir, entry)); err == nil {
			return prefix + "python " + entry
		}
	}
	return ""
}

func pythonTestCommand(dir string) string {
	prefix := pythonCommandPrefix(dir)
	if prefix == "" {
		return ""
	}
	return prefix + "pytest"
}
