package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// checkboxLineRE matches a completion checkbox line and captures the three
// pieces around its state, so only the state itself is ever replaced.
//
// Anchored to a line that *starts* with a list marker so a checkbox written
// inside a code fence or a quote is not mistaken for the real one.
var checkboxLineRE = regexp.MustCompile(`(?m)^(\s*-\s*\[)([ xX])(\].*)$`)

// SetComplete ticks or unticks a note's checkbox in place.
//
// Everything about this function is arranged around one rule: **never
// reconstruct the file.** The note holds the user's study material, this app
// understands perhaps a tenth of what is in it, and rewriting from parsed
// fields would quietly discard the rest. So the only edit made is a
// single-character substitution inside the one matching line — every other
// byte, including line endings, trailing whitespace and anything this parser
// does not understand, is carried through untouched.
//
// Returns whether the file changed. Ticking an already-ticked box is a no-op
// rather than an error: the UI and the vault can disagree if Obsidian was
// edited a moment ago, and the honest resolution is that the requested state
// is now the state.
func SetComplete(path string, complete bool) (changed bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}

	loc := checkboxLineRE.FindSubmatchIndex(raw)
	if loc == nil {
		return false, fmt.Errorf("%s has no completion checkbox to set",
			filepath.Base(path))
	}

	// Group 2 is the single character between the brackets.
	stateStart, stateEnd := loc[4], loc[5]
	current := string(raw[stateStart:stateEnd])
	want := " "
	if complete {
		want = "x"
	}
	// EqualFold covers the whole space: the regex only ever captures " ", "x"
	// or "X", so a case-insensitive match against the target is exactly the
	// "already in this state" test.
	if strings.EqualFold(current, want) {
		return false, nil
	}

	updated := make([]byte, 0, len(raw))
	updated = append(updated, raw[:stateStart]...)
	updated = append(updated, want...)
	updated = append(updated, raw[stateEnd:]...)

	if err := writeAtomic(path, updated); err != nil {
		return false, err
	}
	return true, nil
}

// writeAtomic writes via a temp file in the same directory, then renames.
//
// Obsidian may have this file open and is watching it for changes. A partial
// write — the process dying mid-save, the disk filling — would leave a
// truncated note, and a truncated note is lost work. Rename is atomic on the
// same filesystem, so a reader sees either the old file or the new one and
// never a half-written one.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	// Preserve the original mode rather than assuming 0644.
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	tmp, err := os.CreateTemp(dir, ".excelplan-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename has succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	// Sync before rename: a rename that lands before the data reaches disk
	// gives a note that is atomically empty, which is not an improvement.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("setting mode: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", filepath.Base(path), err)
	}
	return nil
}

// FindBySlug locates a note by its WxDy code.
//
// The API takes a slug, never a path — a path from a client is a directory
// traversal waiting to happen, and the slug is the vault's own identifier
// anyway.
func FindBySlug(root, slug string) (Note, error) {
	notes, err := ReadAll(root)
	if err != nil {
		return Note{}, err
	}
	for _, note := range notes {
		if strings.EqualFold(note.Slug, slug) {
			return note, nil
		}
	}
	return Note{}, fmt.Errorf("no case named %s", slug)
}
