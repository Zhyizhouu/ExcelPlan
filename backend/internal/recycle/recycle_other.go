//go:build !windows

package recycle

import "errors"

// The vault lives on a Windows machine. Elsewhere there is no Recycle Bin to
// use, and deleting outright is the one thing this package exists to avoid.
func move(string) error { return errors.ErrUnsupported }
