package tmux

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
)

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
