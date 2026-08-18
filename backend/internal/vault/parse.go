package vault

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// frontmatter holds the four scalars a day note needs read. The real notes
// carry more (subject, tags, status), left unread on purpose — this package
// renders progress, not the notes themselves, and a field nobody reads is a
// field that cannot drift out of sync with what it feeds.
type frontmatter struct {
	Title   string
	Phase   string
	Week    int
	Created string
}

// scalarRE matches "key: value", taking everything after the *first* colon.
//
// Deliberately not a YAML parse. These are hand-edited notes, and one of them
// really does read `title: W22D2 - Solver: balance the roster` — an unquoted
// second colon, which is invalid YAML and would fail the whole file. Every
// field read here is a flat scalar, so a strict parser buys nothing and costs
// the app its ability to read a vault a human actually typed.
//
// Lines that are not `key: value` (list items under `tags:`, blank lines) do
// not match and are skipped.
var scalarRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$`)

// parseFrontmatter reads the scalars out of a frontmatter block.
func parseFrontmatter(block []byte) frontmatter {
	var fm frontmatter
	sc := bufio.NewScanner(strings.NewReader(string(block)))
	for sc.Scan() {
		m := scalarRE.FindStringSubmatch(strings.TrimRight(sc.Text(), "\r"))
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		value = strings.Trim(value, `"'`) // tolerate quoted scalars either way
		switch strings.ToLower(m[1]) {
		case "title":
			fm.Title = value
		case "phase":
			fm.Phase = value
		case "week":
			if n, err := strconv.Atoi(value); err == nil {
				fm.Week = n
			}
		case "created":
			fm.Created = value
		}
	}
	return fm
}

// slugRE pulls "W1D1" etc. out of a filename like "W1D1 - Keyboard only.md".
var slugRE = regexp.MustCompile(`^(W\d+D\d+)`)

// checkboxRE matches the single completion checkbox every day note ends with,
// in either state: "- [ ] W1D1 complete" or "- [x] W1D1 complete".
var checkboxRE = regexp.MustCompile(`^-\s*\[([ xX])\]`)

// ReadNote parses one day note. A file with no slug in its name or no
// frontmatter block is not a day note and is reported as such rather than
// silently producing a zero-value Note that would misrender as "Phase 0, Week
// 0, incomplete" — indistinguishable from a real one.
func ReadNote(path string) (Note, error) {
	base := filepath.Base(path)
	slug := slugRE.FindString(base)
	if slug == "" {
		return Note{}, fmt.Errorf("%s: filename does not start with a WxDy slug", base)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Note{}, fmt.Errorf("%s: %w", base, err)
	}

	block, body, err := splitFrontmatter(raw)
	if err != nil {
		return Note{}, fmt.Errorf("%s: %w", base, err)
	}

	meta := parseFrontmatter(block)
	created, _ := time.Parse("2006-01-02", meta.Created) // zero time if unparsable
	heading, fields := parseBody(body)

	return Note{
		Slug:     slug,
		Title:    meta.Title,
		Phase:    meta.Phase,
		Week:     meta.Week,
		Created:  created,
		Complete: findCheckbox(body),
		Path:     path,
		Heading:  heading,
		Fields:   fields,
	}, nil
}

var (
	headingRE = regexp.MustCompile(`^#\s+(.+)$`)
	// fieldRE matches "**Objective:** build shortcut muscle memory." — the
	// shape every case in the vault uses for its content.
	fieldRE = regexp.MustCompile(`^\*\*(.+?):\*\*\s*(.*)$`)
	// wikilinkRE matches [[Target]] and [[Target|Alias]]. Rendering the raw
	// syntax would show Obsidian's plumbing to a reader who cannot click it.
	wikilinkRE = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
)

// parseBody pulls the heading and the case's labelled fields out of a note.
//
// Deliberately not a general markdown parse. These notes have one shape, and
// reading exactly that shape means the UI can lay the fields out itself —
// label beside value — instead of rendering a wall of bold text.
func parseBody(body []byte) (heading string, fields []Field) {
	sc := bufio.NewScanner(strings.NewReader(string(body)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // some Do: lines are long
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if heading == "" {
			if m := headingRE.FindStringSubmatch(line); m != nil {
				heading = cleanLinks(m[1])
				continue
			}
		}
		if m := fieldRE.FindStringSubmatch(line); m != nil {
			fields = append(fields, Field{
				Label: strings.TrimSpace(m[1]),
				Value: cleanLinks(strings.TrimSpace(m[2])),
			})
		}
	}
	return heading, fields
}

// cleanLinks turns [[Target|Alias]] into Alias and [[Target]] into Target.
func cleanLinks(s string) string {
	return wikilinkRE.ReplaceAllStringFunc(s, func(match string) string {
		m := wikilinkRE.FindStringSubmatch(match)
		if m[2] != "" {
			return m[2]
		}
		return m[1]
	})
}

// splitFrontmatter separates the leading "---\n...\n---" YAML block from the
// rest of the note. Obsidian's own convention, so this is the format every
// note in the vault already follows — nothing here is inventing a schema.
func splitFrontmatter(raw []byte) (fm, body []byte, err error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return nil, nil, fmt.Errorf("no frontmatter block")
	}
	rest := text[strings.Index(text, "\n")+1:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return nil, nil, fmt.Errorf("frontmatter block never closes")
	}
	return []byte(rest[:end]), []byte(rest[end+4:]), nil
}

// findCheckbox reports whether the note's completion checkbox is ticked.
//
// Matches on the checkbox syntax alone, not the trailing "complete" text —
// day notes are free-form otherwise, and asserting the label wastes matches
// no note has ever failed on that one line was written for.
func findCheckbox(body []byte) bool {
	sc := bufio.NewScanner(strings.NewReader(string(body)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := checkboxRE.FindStringSubmatch(line); m != nil {
			return m[1] != " "
		}
	}
	return false
}

// ReadAll walks every WxDy note under root (a "Phase */*.md" glob one level
// deep) and returns them in phase, then week, then slug order — the order the
// plan is meant to be worked, and the order NextUp below assumes.
func ReadAll(root string) ([]Note, error) {
	matches, err := filepath.Glob(filepath.Join(root, "Phase *", "*.md"))
	if err != nil {
		return nil, err
	}

	notes := make([]Note, 0, len(matches))
	for _, path := range matches {
		note, err := ReadNote(path)
		if err != nil {
			return nil, err // one unreadable note invalidates the whole progress read
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		a, b := notes[i], notes[j]
		if oa, ob := phaseOrder(a.Phase), phaseOrder(b.Phase); oa != ob {
			return oa < ob
		}
		if a.Week != b.Week {
			return a.Week < b.Week
		}
		return a.Slug < b.Slug
	})
	return notes, nil
}
