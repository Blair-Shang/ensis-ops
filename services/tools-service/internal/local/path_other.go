//go:build !windows

package local

import "path/filepath"

func canonical(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
