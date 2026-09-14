package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// A day note as the real vault writes it — frontmatter plus a body ending in
// exactly one completion checkbox. Fixtures are copied verbatim from
// Fred/Excel Mastery rather than paraphrased, so a change to the real format
// breaks this test before it breaks the app.
const uncheckedNote = `---
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

**Objective:** build shortcut muscle memory.

- [ ] W1D1 complete
`

const checkedNote = `---
title: W1D2 - Mixed referencing
phase: 0
week: 1
created: 2026-08-11
---

- [x] W1D2 complete
`

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadNote(t *testing.T) {
	dir := t.TempDir()

	t.Run("an unchecked box parses as incomplete", func(t *testing.T) {
		path := writeFixture(t, dir, "W1D1 - Keyboard only.md", uncheckedNote)
		note, err := ReadNote(path)
		if err != nil {
			t.Fatal(err)
		}
		if note.Slug != "W1D1" || note.Complete {
			t.Fatalf("got slug=%q complete=%v, want W1D1/false", note.Slug, note.Complete)
		}
		if note.Title != "W1D1 - Keyboard only" || note.Phase != "0" || note.Week != 1 {
			t.Fatalf("frontmatter not read correctly: %+v", note)
		}
		if note.Created.Format("2006-01-02") != "2026-08-10" {
			t.Fatalf("created = %v", note.Created)
		}
	})

	t.Run("a checked box, including case, parses as complete", func(t *testing.T) {
		path := writeFixture(t, dir, "W1D2 - Mixed referencing.md", checkedNote)
		note, err := ReadNote(path)
		if err != nil {
			t.Fatal(err)
		}
		if !note.Complete {
			t.Fatal("[x] was not read as complete")
		}
	})

	t.Run("a filename with no WxDy slug is refused, not silently zero-valued", func(t *testing.T) {
		path := writeFixture(t, dir, "notes.md", uncheckedNote)
		if _, err := ReadNote(path); err == nil {
			t.Fatal("expected an error for a non-day-note filename")
		}
	})

	// Taken verbatim from Phase 8/W22D2, which really does carry an unquoted
	// second colon in its title. Strict YAML rejects the whole file; a plan
	// tracker that falls over because one title has a colon in it is useless
	// against a vault a human types into.
	t.Run("an unquoted colon in a title does not break the note", func(t *testing.T) {
		path := writeFixture(t, dir, "W22D2 - Solver optimisation.md", `---
title: W22D2 - Solver: balance the roster
phase: 8
week: 22
created: 2026-08-10
---

- [ ] W22D2 complete
`)
		note, err := ReadNote(path)
		if err != nil {
			t.Fatalf("a colon in a title should not fail the parse: %v", err)
		}
		if note.Title != "W22D2 - Solver: balance the roster" {
			t.Fatalf("title = %q, want everything after the first colon kept", note.Title)
		}
		if note.Phase != "8" || note.Week != 22 {
			t.Fatalf("later fields lost: %+v", note)
		}
	})

	t.Run("quoted scalars are unwrapped", func(t *testing.T) {
		path := writeFixture(t, dir, "W3D3 - Quoted.md", `---
title: "W3D3 - Quoted title"
phase: "1"
week: 3
created: 2026-08-10
---

- [ ] W3D3 complete
`)
		note, err := ReadNote(path)
		if err != nil {
			t.Fatal(err)
		}
		if note.Title != "W3D3 - Quoted title" || note.Phase != "1" {
			t.Fatalf("quotes not stripped: %+v", note)
		}
	})

	t.Run("a note missing its frontmatter block is refused", func(t *testing.T) {
		path := writeFixture(t, dir, "W9D9 - No frontmatter.md", "# just a heading\n- [ ] W9D9 complete\n")
		if _, err := ReadNote(path); err == nil {
			t.Fatal("expected an error for a missing frontmatter block")
		}
	})
}

func TestFindCheckbox(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"lowercase x", "- [x] done", true},
		{"uppercase X", "- [X] done", true},
		{"empty box", "- [ ] done", false},
		{"no checkbox at all", "just prose, no box", false},
		{"checkbox mid-document, other lines ignored", "some text\n- [x] W1D1 complete\nmore text", true},
		{"indented checkbox still matches", "  - [x] done", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := findCheckbox([]byte(tc.body)); got != tc.want {
				t.Fatalf("findCheckbox(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

// The real vault, read end to end. This is the test that matters most: it
// fails the moment the actual Excel Mastery notes stop matching what the
// parser assumes, rather than only ever exercising fixtures that were written
// to agree with the code.
func TestReadAll_RealVault(t *testing.T) {
	root := `D:\Abner\Obsidian\Fred\Excel Mastery`
	if _, err := os.Stat(root); err != nil {
		t.Skipf("real vault not present at %s: %v", root, err)
	}

	notes, err := ReadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 127 {
		t.Fatalf("got %d notes, want 127 (the vault's own stated total)", len(notes))
	}

	// Sorted by phase order, then week, then slug — the order the plan is
	// worked in. Phase order is numeric, so "2-3" must land between 1 and 4.
	for i := 1; i < len(notes); i++ {
		a, b := notes[i-1], notes[i]
		oa, ob := phaseOrder(a.Phase), phaseOrder(b.Phase)
		inOrder := oa < ob ||
			(oa == ob && a.Week < b.Week) ||
			(oa == ob && a.Week == b.Week && a.Slug < b.Slug)
		if !inOrder {
			t.Fatalf("notes out of order at %d: %s (phase %s wk %d) before %s (phase %s wk %d)",
				i, a.Slug, a.Phase, a.Week, b.Slug, b.Phase, b.Week)
		}
	}

	// The merged folder is real and must survive the round trip.
	var merged int
	for _, n := range notes {
		if n.Phase == "2-3" {
			merged++
		}
	}
	if merged != 29 {
		t.Fatalf("got %d notes in the merged 2-3 phase, want 29", merged)
	}

	slugs := make(map[string]bool, len(notes))
	for _, n := range notes {
		if slugs[n.Slug] {
			t.Fatalf("duplicate slug %s", n.Slug)
		}
		slugs[n.Slug] = true
	}
}

func TestReadPhases_RealHubNote(t *testing.T) {
	path := `D:\Abner\Obsidian\Fred\Excel Mastery\Excel_Mastery.md`
	if _, err := os.Stat(path); err != nil {
		t.Skipf("real hub note not present: %v", err)
	}

	phases, err := ReadPhases(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != 9 { // Phase 0 through Phase 8, each with its own header
		t.Fatalf("got %d phase headers, want 9", len(phases))
	}
	if phases["0"].Name != "Calibration & Speed" {
		t.Fatalf("phase 0 = %+v", phases["0"])
	}
	// Both halves of the merged folder are named separately in the hub note,
	// which is what lets describe() join them.
	if phases["2"].Name == "" || phases["3"].Name == "" {
		t.Fatalf("expected separate headers for phases 2 and 3, got %+v / %+v",
			phases["2"], phases["3"])
	}
}

// End to end against the real files: the vault's own stated total, correctly
// grouped, with a next-up that actually exists.
func TestRoll_RealVault(t *testing.T) {
	root := `D:\Abner\Obsidian\Fred\Excel Mastery`
	if _, err := os.Stat(root); err != nil {
		t.Skipf("real vault not present: %v", err)
	}

	notes, err := ReadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	headers, err := ReadPhases(filepath.Join(root, "Excel_Mastery.md"))
	if err != nil {
		t.Fatal(err)
	}

	got := Roll(notes, headers)
	if got.Total != 127 {
		t.Fatalf("total = %d, want 127", got.Total)
	}
	if len(got.Phases) != 8 { // 9 headers, but 2 and 3 merge into one key
		t.Fatalf("got %d phases, want 8 (2-3 is one folder)", len(got.Phases))
	}
	for i := 1; i < len(got.Phases); i++ {
		if got.Phases[i-1].Order > got.Phases[i].Order {
			t.Fatalf("phases out of order: %+v then %+v",
				got.Phases[i-1].Key, got.Phases[i].Key)
		}
	}
	if got.Completed < got.Total && got.NextUp == nil {
		t.Fatal("work remains but NextUp is nil")
	}
}
