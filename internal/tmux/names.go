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

func WindowName(workspace string) string {
	return "ws:" + sanitize(workspace)
}

func GroupOptionKey(group string) string {
	return "@wts_active_" + sanitize(group)
}

func LastSelectedOptionKey() string {
	return "@wts_last_selected"
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
