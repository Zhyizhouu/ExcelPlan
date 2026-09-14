//go:build windows

package recycle

import (
	"fmt"
	"syscall"
	"unsafe"
)

// shFileOpStruct is SHFILEOPSTRUCTW; Go's natural alignment matches it on 64-bit.
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete          = 0x3
	fofSilent         = 0x4
	fofNoConfirmation = 0x10
	fofAllowUndo      = 0x40
	fofNoErrorUI      = 0x400
)

var shFileOperation = syscall.NewLazyDLL("shell32.dll").NewProc("SHFileOperationW")

// move uses the shell's own delete with undo allowed, which is what sends a
// file to the Recycle Bin. Every UI flag is off: this runs inside a server, and
// a dialog nobody can see would hang the request.
func move(path string) error {
	from, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	from = append(from, 0) // the list of paths ends with a second null

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	if code, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op))); code != 0 {
		return fmt.Errorf("shell error 0x%x (is the file open in Excel?)", code)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("the operation was aborted")
	}
	return nil
}
