// Package root finds the directory mdpls should serve.
package root

import (
	"os"
	"path/filepath"
)

// Find returns the nearest ancestor of dir (or dir itself) that contains a
// .git entry (directory, or file for worktrees/submodules), else dir.
func Find(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}
