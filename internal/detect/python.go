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

	runner := pythonRunner(dir)
	procs = append(procs, Process{
		Name:    "dev",
		Command: runner + " run dev",
	})
	procs = append(procs, Process{
		Name:    "test",
		Command: runner + " run test",
	})

	return &Result{Type: "python", Processes: procs}, nil
}

func pythonRunner(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "poetry.lock")); err == nil {
		return "poetry"
	}
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); err == nil {
		return "uv"
	}
	return "python -m"
}
