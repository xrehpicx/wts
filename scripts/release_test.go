package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests use real temporary Git repositories and a local bare origin.
// Only the GitHub CLI is replaced, so no credentials or network are needed.
type releaseRepo struct {
	dir     string
	remote  string
	scripts string
	output  string
	ghLog   string
	env     []string
}

func newReleaseRepo(t *testing.T) *releaseRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("release scripts run on Unix hosts and GitHub's Ubuntu runner")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	scripts, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	r := &releaseRepo{
		dir: filepath.Join(root, "work"), remote: filepath.Join(root, "origin.git"),
		scripts: scripts, output: filepath.Join(root, "output"), ghLog: filepath.Join(root, "gh.log"),
	}
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{r.dir, bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" >> "$GH_LOG"
if [ "${GH_FAIL:-}" = true ]; then
  echo 'GitHub API unavailable' >&2
  exit 1
fi
if [ "$1" = api ]; then
  printf '%s' "${GH_STATE:-}"
fi
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	r.env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+root, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Release Test", "GIT_AUTHOR_EMAIL=release@example.invalid",
		"GIT_COMMITTER_NAME=Release Test", "GIT_COMMITTER_EMAIL=release@example.invalid",
		"GITHUB_REF=refs/heads/main", "GITHUB_OUTPUT="+r.output, "GH_LOG="+r.ghLog,
		"GH_STATE=", "GH_FAIL=false",
	)
	r.git(t, "init", "--bare", r.remote)
	r.git(t, "init", "-b", "main")
	r.git(t, "config", "commit.gpgsign", "false")
	r.git(t, "config", "tag.gpgsign", "false")
	r.git(t, "remote", "add", "origin", r.remote)
	r.git(t, "commit", "--allow-empty", "-m", "Initial commit")
	r.push(t)
	return r
}

func (r *releaseRepo) command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	cmd.Env = r.env
	return cmd
}

func (r *releaseRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	out, err := r.command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *releaseRepo) push(t *testing.T) {
	t.Helper()
	r.git(t, "push", "--tags", "origin", "main")
}

func (r *releaseRepo) script(t *testing.T, name string, wantError string, args ...string) string {
	t.Helper()
	out, err := r.command("bash", append([]string{filepath.Join(r.scripts, name)}, args...)...).CombinedOutput()
	if wantError == "" {
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	} else if err == nil || !strings.Contains(string(out), wantError) {
		t.Fatalf("%s: error = %v, output = %s; want error containing %q", name, err, out, wantError)
	}
	return string(out)
}

func readReleaseFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReleasePrepareSelectsSemanticVersion(t *testing.T) {
	for _, tt := range []struct {
		name string
		bump string
		tags []string
		want string
	}{
		{"first patch", "patch", nil, "v0.0.1"},
		{"first minor", "minor", nil, "v0.1.0"},
		{"ignore assets and prereleases", "patch", []string{"v0.3.1", "vdemo-assets.1.1", "demo-assets", "v9.0.0-rc.1", "v01.0.0"}, "v0.3.2"},
		{"minor", "minor", []string{"v0.3.1"}, "v0.4.0"},
		{"numeric ordering", "patch", []string{"v0.9.9", "v0.10.2"}, "v0.10.3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			for _, tag := range tt.tags {
				r.git(t, "tag", tag)
			}
			r.git(t, "commit", "--allow-empty", "-m", "Feature")
			r.push(t)
			revision := r.git(t, "rev-parse", "HEAD")
			r.script(t, "release-prepare.sh", "", tt.bump, revision)
			output := readReleaseFile(t, r.output)
			for _, want := range []string{"released=true\n", "tag=" + tt.want + "\n", "revision=" + revision + "\n"} {
				if !strings.Contains(output, want) {
					t.Errorf("outputs %q missing %q", output, want)
				}
			}
			if got := r.git(t, "rev-parse", tt.want+"^{commit}"); got != revision {
				t.Errorf("tag points to %s; want %s", got, revision)
			}
			if got := r.git(t, "ls-remote", "origin", "refs/tags/"+tt.want+"^{}"); !strings.HasPrefix(got, revision) {
				t.Errorf("remote tag does not point to tested revision: %s", got)
			}
		})
	}
}

func TestReleasePrepareRetriesSameTag(t *testing.T) {
	for _, tt := range []struct {
		name, state, released string
	}{
		{"missing release", "", "true"},
		{"draft release", "true", "true"},
		{"published release", "false", "false"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.git(t, "tag", "v0.3.0")
			r.git(t, "commit", "--allow-empty", "-m", "Release")
			r.git(t, "tag", "v0.3.1")
			r.git(t, "tag", "vdemo-assets.1.1")
			r.push(t)
			r.env = append(r.env, "GH_STATE="+tt.state)
			before := r.git(t, "show-ref", "--tags")
			r.script(t, "release-prepare.sh", "", "minor", r.git(t, "rev-parse", "HEAD"))
			output := readReleaseFile(t, r.output)
			for _, want := range []string{"released=" + tt.released + "\n", "tag=v0.3.1\n", "previous_tag=v0.3.0\n"} {
				if !strings.Contains(output, want) {
					t.Errorf("outputs %q missing %q", output, want)
				}
			}
			if got := r.git(t, "show-ref", "--tags"); got != before {
				t.Errorf("retry changed tags: %s", got)
			}
		})
	}
}

func TestReleasePrepareRejectsUnsafeState(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		setup      func(*testing.T, *releaseRepo)
	}{
		{"wrong branch", "must run from main", func(_ *testing.T, r *releaseRepo) { r.env = append(r.env, "GITHUB_REF=refs/heads/feature") }},
		{"API failure", "GitHub API unavailable", func(_ *testing.T, r *releaseRepo) { r.env = append(r.env, "GH_FAIL=true") }},
		{"published release without tag", "refusing to retag", func(_ *testing.T, r *releaseRepo) { r.env = append(r.env, "GH_STATE=false") }},
		{"remote behind", "Main changed", func(t *testing.T, r *releaseRepo) { r.git(t, "commit", "--allow-empty", "-m", "Unpushed") }},
		{"divergent release", "not an ancestor", func(t *testing.T, r *releaseRepo) {
			r.git(t, "checkout", "-b", "other")
			r.git(t, "commit", "--allow-empty", "-m", "Other release")
			r.git(t, "tag", "v99.0.0")
			r.git(t, "checkout", "main")
			r.push(t)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.git(t, "tag", "v0.3.1")
			r.git(t, "commit", "--allow-empty", "-m", "Feature")
			r.push(t)
			tt.setup(t, r)
			before := r.git(t, "show-ref", "--tags")
			r.script(t, "release-prepare.sh", tt.want, "patch", r.git(t, "rev-parse", "HEAD"))
			if got := r.git(t, "show-ref", "--tags"); got != before {
				t.Errorf("failed preparation changed tags: %s", got)
			}
		})
	}
}

func TestReleaseDispatchPinsRevisionWithoutTagging(t *testing.T) {
	t.Parallel()
	r := newReleaseRepo(t)
	revision := r.git(t, "rev-parse", "HEAD")
	r.script(t, "release.sh", "", "minor")
	want := "workflow\nrun\nrelease.yml\n--ref\nmain\n-f\nbump=minor\n-f\nrevision=" + revision + "\n"
	if got := readReleaseFile(t, r.ghLog); got != want {
		t.Fatalf("GitHub invocation = %q; want %q", got, want)
	}
	if tags := r.git(t, "tag", "--list"); tags != "" {
		t.Errorf("dispatch unexpectedly created tags: %s", tags)
	}
}

func TestReleaseDispatchRejectsUnsafeState(t *testing.T) {
	for _, tt := range []struct {
		name, bump, want string
		setup            func(*testing.T, *releaseRepo)
	}{
		{"invalid bump", "major", "must be patch or minor", nil},
		{"wrong branch", "patch", "must be created from main", func(t *testing.T, r *releaseRepo) { r.git(t, "checkout", "-b", "feature") }},
		{"detached", "patch", "current branch: detached", func(t *testing.T, r *releaseRepo) { r.git(t, "checkout", "--detach") }},
		{"unpushed", "patch", "must match origin/main", func(t *testing.T, r *releaseRepo) { r.git(t, "commit", "--allow-empty", "-m", "Unpushed") }},
		{"dirty", "patch", "must be clean", func(t *testing.T, r *releaseRepo) {
			if err := os.WriteFile(filepath.Join(r.dir, "untracked"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			if tt.setup != nil {
				tt.setup(t, r)
			}
			r.script(t, "release.sh", tt.want, tt.bump)
			if _, err := os.Stat(r.ghLog); !os.IsNotExist(err) {
				t.Fatalf("unsafe dispatch called GitHub CLI: %v", err)
			}
		})
	}
}

func TestReleasePrepareRejectsUnverifiedCheckout(t *testing.T) {
	for _, tt := range []struct {
		name, bump, revision, want string
	}{
		{"invalid bump", "major", "HEAD", "must be patch or minor"},
		{"missing revision", "patch", "", "does not match the verified checkout"},
		{"symbolic revision", "patch", "HEAD", "does not match the verified checkout"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.script(t, "release-prepare.sh", tt.want, tt.bump, tt.revision)
			if _, err := os.Stat(r.ghLog); !os.IsNotExist(err) {
				t.Fatalf("unverified checkout called GitHub CLI: %v", err)
			}
		})
	}
}
