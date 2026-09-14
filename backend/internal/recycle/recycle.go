// Package recycle moves a file to the Recycle Bin instead of deleting it, so
// anything it removes can still be restored by hand.
package recycle

import (
	"errors"
	"fmt"
	"os"
)

// Move sends path to the Recycle Bin and confirms it has left its folder.
func Move(path string) error {
	if err := move(path); err != nil {
		return fmt.Errorf("moving %s to the Recycle Bin: %w", path, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("moving %s to the Recycle Bin: the file is still there", path)
	}
	return nil
}
