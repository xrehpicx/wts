package tmux

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
)

// ProcessPaneTitle returns the tmux pane title for a managed process.
func ProcessPaneTitle(processName string) string {
	return "wts:" + processName
}

// ProcessFromPaneTitle extracts the process name from a wts pane title.
// Returns empty string if the title is not a wts-managed pane.
func ProcessFromPaneTitle(title string) string {
	if strings.HasPrefix(title, "wts:") {
		return title[4:]
	}
	return ""
}

func SessionName(repoRoot string) string {
	base := sanitize(filepath.Base(repoRoot))
	hash := sha1.Sum([]byte(repoRoot))
	return fmt.Sprintf("wts_%x_%s", hash[:4], base)
}

func WindowName(worktreeDir string) string {
	base := sanitize(filepath.Base(worktreeDir))
	hash := sha1.Sum([]byte(filepath.Clean(worktreeDir)))
	return fmt.Sprintf("ws_%x_%s", hash[:4], base)
}

func ActiveWorktreeOptionKey() string {
	return "@wts_active_worktree"
}

func ActiveProcessOptionKey() string {
	return "@wts_active_process"
}

func ProcessOptionKey(worktreeDir string) string {
	hash := sha1.Sum([]byte(filepath.Clean(worktreeDir)))
	return fmt.Sprintf("@wts_process_%x", hash[:6])
}

// IsShellCommand returns true if the command name looks like a shell.
// Used to detect if a pane's foreground has returned to its shell (process exited).
func IsShellCommand(cmd string) bool {
	switch cmd {
	case "fish", "bash", "zsh", "sh", "dash", "ksh", "csh", "tcsh", "nu", "elvish", "ion":
		return true
	}
	return false
}

func sanitize(value string) string {
	if value == "" {
		return "default"
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
