// Package vault reads the Excel Mastery study plan out of the Obsidian vault
// it already lives in, rather than duplicating it into a database.
//
// The vault is the source of truth for what to study and in what order; this
// package only reads it. The one thing it is not the source of truth for is
// *when* a day was finished — the checkbox in each note is a bare boolean, no
// timestamp — which is why completion dates live in a separate log package
// instead of being invented here.
package vault

import (
	"strconv"
	"strings"
	"time"
)

// Note is one day's plan: what to do, and whether it is done.
type Note struct {
	// Slug is the WxDy code the whole vault is keyed by — filenames and
	// wikilinks both use it, the frontmatter is silent on it. Derived from the
	// filename rather than stored, so renaming a file cannot desync it from
	// what links to it.
	Slug  string
	Title string

	// Phase is the raw frontmatter value, kept as written.
	//
	// It is a string and not a number because 25 of the real notes say
	// `phase: 2-3`: the vault merges Phases 2 and 3 into one folder that
	// shares its dataset, while the hub note still names them separately.
	// Parsing that into an int would either fail or silently pick one half.
	Phase    string
	Week     int
	Created  time.Time
	Complete bool
	Path     string

	// Heading is the note's `# ...` line, which is often more specific than
	// the frontmatter title — "W7D1 - First pivot (workload by assistant)"
	// against a title of "W7D1 - First pivot".
	Heading string
	// Fields are the `**Label:** value` lines that make up the case itself:
	// Objective, Data, Do, Deliverable, Stretch. Kept as an ordered list
	// rather than a map because which fields a note has varies — some carry no
	// Objective, most carry no Stretch — and their order on the page is the
	// order they were written in.
	Fields []Field
}

// Field is one `**Label:** value` line from a case.
type Field struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Phase is a span of the plan as the hub note describes it.
//
// Key is the frontmatter value the day notes group by ("0", "2-3"); Name and
// WeekRange come from the hub note's headers, which are the only place a
// phase is described in words. A merged phase takes both halves' names.
type Phase struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// WeekRange is display text ("Weeks 5-9"), not a parsed span. Named to
	// stay distinct from PhaseProgress.Weeks, which is the actual list of
	// weeks — an embedded field and an outer one sharing a name compiles
	// fine and silently shadows, which is worse than either being wrong.
	WeekRange string `json:"weekRange"`
	// Order sorts phases by their leading number, so "2-3" lands between "1"
	// and "4" rather than wherever string comparison would put it.
	Order int `json:"order"`
}

// phaseOrder reads the leading integer out of a phase key, so "2-3" sorts as
// 2. An unparsable key sorts last rather than first — a malformed phase
// showing up at the top of the plan would read as the place to start.
func phaseOrder(key string) int {
	head, _, _ := strings.Cut(key, "-")
	n, err := strconv.Atoi(strings.TrimSpace(head))
	if err != nil {
		return 1 << 30
	}
	return n
}

// phaseParts splits a merged key into the individual numbers the hub note
// names separately: "2-3" -> ["2", "3"], "0" -> ["0"].
func phaseParts(key string) []string {
	parts := strings.Split(key, "-")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
