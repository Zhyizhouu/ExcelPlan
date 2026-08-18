//go:build !windows

package openfile

import (
	"os/exec"
	"runtime"
)

// command opens path with the desktop's file handler. Present so the package
// builds and behaves sensibly off Windows; the vault this app reads lives on a
// Windows machine, so this path is the untested one.
func command(path string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path)
	}
	return exec.Command("xdg-open", path)
}
