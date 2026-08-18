package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The whole file, so the tests below can assert that everything except one
// character survives a write untouched.
const fullNote = `---
title: W1D1 - Keyboard only
subject: Excel
phase: 0
week: 1
tags:
  - plan
status: active
created: 2026-08-10
---

> [!info] Phase 0 · Week 1 · Day 1
> Root: [[Excel_Mastery]] · Prev: none · Next: [[W1D2 - Mixed referencing]]

# W1D1 - Keyboard only

**Objective:** build shortcut muscle memory; prove your real starting speed.
**Data:** TeachingScheduleQuery.
**Deliverable:** the extract rebuilt, mouse untouched.

- [ ] W1D1 complete
`

func noteAt(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "W1D1 - Keyboard only.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSetComplete(t *testing.T) {
	t.Run("ticks an unticked box", func(t *testing.T) {
		path := noteAt(t, fullNote)
		changed, err := SetComplete(path, true)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if !strings.Contains(read(t, path), "- [x] W1D1 complete") {
			t.Fatalf("box not ticked:\n%s", read(t, path))
		}
	})

	t.Run("unticks a ticked box", func(t *testing.T) {
		path := noteAt(t, strings.Replace(fullNote, "- [ ]", "- [x]", 1))
		changed, err := SetComplete(path, false)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if !strings.Contains(read(t, path), "- [ ] W1D1 complete") {
			t.Fatalf("box not cleared:\n%s", read(t, path))
		}
	})

	// The rule the whole design rests on. If anything but that one character
	// moves, the app is editing study notes it does not understand.
	t.Run("changes exactly one character and nothing else", func(t *testing.T) {
		path := noteAt(t, fullNote)
		if _, err := SetComplete(path, true); err != nil {
			t.Fatal(err)
		}

		after := read(t, path)
		if len(after) != len(fullNote) {
			t.Fatalf("length changed: %d -> %d", len(fullNote), len(after))
		}
		diffs := 0
		for i := range after {
			if after[i] != fullNote[i] {
				diffs++
			}
		}
		if diffs != 1 {
			t.Fatalf("%d bytes differ, want exactly 1", diffs)
		}
	})

	t.Run("setting the state it is already in is a silent no-op", func(t *testing.T) {
		path := noteAt(t, fullNote)
		changed, err := SetComplete(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("reported a change when nothing needed changing")
		}
		if read(t, path) != fullNote {
			t.Fatal("file was rewritten despite no change being needed")
		}
	})

	t.Run("an already-ticked box accepts a tick without rewriting", func(t *testing.T) {
		ticked := strings.Replace(fullNote, "- [ ]", "- [X]", 1) // uppercase
		path := noteAt(t, ticked)
		changed, err := SetComplete(path, true)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("uppercase X should already count as complete")
		}
		if read(t, path) != ticked {
			t.Fatal("uppercase X was needlessly rewritten to lowercase")
		}
	})

	// Windows notes are CRLF. A naive line-split-and-rejoin would silently
	// convert the whole file to LF, which shows up as every line changed in
	// git and as a modified file in Obsidian.
	t.Run("CRLF line endings survive", func(t *testing.T) {
		crlf := strings.ReplaceAll(fullNote, "\n", "\r\n")
		path := noteAt(t, crlf)
		if _, err := SetComplete(path, true); err != nil {
			t.Fatal(err)
		}
		after := read(t, path)
		if strings.Count(after, "\r\n") != strings.Count(crlf, "\r\n") {
			t.Fatalf("CRLF count changed: %d -> %d",
				strings.Count(crlf, "\r\n"), strings.Count(after, "\r\n"))
		}
	})

	t.Run("a note with no checkbox is refused rather than appended to", func(t *testing.T) {
		path := noteAt(t, "---\ntitle: x\n---\n\nno checkbox here\n")
		if _, err := SetComplete(path, true); err == nil {
			t.Fatal("expected an error for a note with no checkbox")
		}
	})

	// Only the first checkbox is the completion box; a case that happens to
	// list sub-tasks must not have its state read from one of those.
	t.Run("only the first checkbox is touched", func(t *testing.T) {
		multi := "---\ntitle: x\n---\n\n- [ ] W1D1 complete\n- [ ] a sub task\n"
		path := noteAt(t, multi)
		if _, err := SetComplete(path, true); err != nil {
			t.Fatal(err)
		}
		after := read(t, path)
		if !strings.Contains(after, "- [x] W1D1 complete") {
			t.Fatal("first box not ticked")
		}
		if !strings.Contains(after, "- [ ] a sub task") {
			t.Fatalf("a later checkbox was also changed:\n%s", after)
		}
	})

	t.Run("file mode is preserved across the atomic replace", func(t *testing.T) {
		path := noteAt(t, fullNote)
		if err := os.Chmod(path, 0o600); err != nil {
			t.Skipf("chmod unsupported here: %v", err)
		}
		before, _ := os.Stat(path)
		if _, err := SetComplete(path, true); err != nil {
			t.Fatal(err)
		}
		after, _ := os.Stat(path)
		if before.Mode() != after.Mode() {
			t.Fatalf("mode changed: %v -> %v", before.Mode(), after.Mode())
		}
	})

	// The temp file is created alongside the note, so a crash must not leave
	// litter in the user's vault folder.
	t.Run("no temp files are left behind", func(t *testing.T) {
		path := noteAt(t, fullNote)
		if _, err := SetComplete(path, true); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".excelplan-") {
				t.Fatalf("left a temp file behind: %s", e.Name())
			}
		}
	})
}

// A round trip through the parser: what SetComplete writes must be what
// ReadNote reads back, or the UI and the file disagree after every tick.
func TestSetCompleteRoundTrip(t *testing.T) {
	path := noteAt(t, fullNote)

	for _, want := range []bool{true, false, true} {
		if _, err := SetComplete(path, want); err != nil {
			t.Fatal(err)
		}
		note, err := ReadNote(path)
		if err != nil {
			t.Fatal(err)
		}
		if note.Complete != want {
			t.Fatalf("wrote complete=%v, read back %v", want, note.Complete)
		}
	}
}
