//go:build windows

package openfile

import (
	"os/exec"
	"syscall"
)

// command opens path through cmd's `start`, which is the only thing on Windows
// that resolves a file to its registered handler.
//
// The empty string is start's title argument. Without it, start reads the
// quoted path as the window title and opens nothing — the path has to be
// quoted, because the vault folder has a space in it.
//
// HideWindow keeps the console cmd would otherwise flash on screen. Saving a
// workbook should look like Excel opening, not like something ran.
func command(path string) *exec.Cmd {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
