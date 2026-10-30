//go:build !windows

package local

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

func canReadVersion(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".bat", ".cmd", ".ps1", ".vbs", ".com":
		return false
	default:
		return true
	}
}

func consoleVersionCommand(ctx context.Context, path string) *exec.Cmd {
	return exec.CommandContext(ctx, path, "-version")
}
