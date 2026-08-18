// Package openfile hands a file to whichever program the desktop has
// registered for it.
//
// Only ever used on the machine the vault lives on, which is the same machine
// the server runs on — this app has no deploy target, so "open it" means what
// it sounds like rather than something a remote client could ask for.
package openfile

import "fmt"

// Open launches path in its default application and returns without waiting.
//
// Fire-and-forget on purpose. The caller is an HTTP handler, and the program
// on the other end is Excel — which stays open for as long as the case takes,
// far longer than a request should. A failure here is worth reporting but is
// never worth failing the save over: the file is already written either way.
func Open(path string) error {
	cmd := command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	// Reaped in the background so the launcher process does not linger as a
	// zombie once it has handed off. Nothing waits on the result.
	go func() { _ = cmd.Wait() }()
	return nil
}
