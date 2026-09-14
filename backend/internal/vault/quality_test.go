package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Quality checks over the real vault.
//
// Separate from parse_test.go on purpose. Those tests ask "does the parser read
// what is there"; these ask "is what is there any good". A note can parse
// perfectly and still be missing the objective the tutor needs, or claim a week
// its slug contradicts — both invisible to a parser test, both things a student
// hits.
//
// Every one of these reports *all* offenders rather than failing on the first,
// because the output is meant to be read as an audit and worked through, not
// fixed one `go test` run at a time.

const realVault = `D:\Abner\Obsidian\Fred\Excel Mastery`

func vaultNotes(t *testing.T) []Note {
	t.Helper()
	if _, err := os.Stat(realVault); err != nil {
		t.Skipf("real vault not present at %s", realVault)
	}
	notes, err := ReadAll(realVault)
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

func fieldsOf(n Note) map[string]string {
	out := make(map[string]string, len(n.Fields))
	for _, f := range n.Fields {
		out[f.Label] = f.Value
	}
	return out
}

// Objective is the case's learning outcome — what you are supposed to walk away
// able to do. The tutor is handed it as briefing, and a case without one is a
// case where both the student and the tutor are inferring the point from the
// instructions.
func TestEveryNoteStatesItsObjective(t *testing.T) {
	notes := vaultNotes(t)

	var missing []string
	for _, n := range notes {
		if strings.TrimSpace(fieldsOf(n)["Objective"]) == "" {
			missing = append(missing, n.Slug)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d of %d notes have no Objective:\n  %s",
			len(missing), len(notes), strings.Join(missing, " "))
	}
}

// Do and Deliverable are the instruction and the finish line. A case missing
// either cannot be started or cannot be known to be done.
func TestEveryNoteHasDoAndDeliverable(t *testing.T) {
	notes := vaultNotes(t)

	for _, required := range []string{"Do", "Deliverable"} {
		var missing []string
		for _, n := range notes {
			if strings.TrimSpace(fieldsOf(n)[required]) == "" {
				missing = append(missing, n.Slug)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%d notes have no %s: %s",
				len(missing), required, strings.Join(missing, " "))
		}
	}
}

var slugParts = regexp.MustCompile(`^W(\d+)D(\d+)$`)

// The slug is the one identifier used everywhere — filenames, links, the API,
// the tutor's threads. Frontmatter that disagrees with it means the same case
// sorts into one week and claims another.
func TestSlugAgreesWithFrontmatter(t *testing.T) {
	notes := vaultNotes(t)

	for _, n := range notes {
		m := slugParts.FindStringSubmatch(n.Slug)
		if m == nil {
			t.Errorf("%s: slug is not WxDy", n.Slug)
			continue
		}
		week, _ := strconv.Atoi(m[1])
		if n.Week != week {
			t.Errorf("%s: frontmatter says week %d, slug says week %d",
				n.Slug, n.Week, week)
		}

		// The folder is the other claim about where a case lives. "Phase 2-3"
		// covers both, so the folder name has to contain the phase, not equal it.
		folder := filepath.Base(filepath.Dir(n.Path))
		if !strings.Contains(folder, n.Phase) {
			t.Errorf("%s: frontmatter phase %q but folder is %q",
				n.Slug, n.Phase, folder)
		}
	}
}

// A week with four days, or six, is a content mistake that nothing else
// notices: the totals still add up, the page still renders, and the plan
// quietly stops being what it says it is.
func TestEveryWeekIsWhollyPresent(t *testing.T) {
	notes := vaultNotes(t)

	days := map[int][]int{}
	for _, n := range notes {
		if m := slugParts.FindStringSubmatch(n.Slug); m != nil {
			week, _ := strconv.Atoi(m[1])
			day, _ := strconv.Atoi(m[2])
			days[week] = append(days[week], day)
		}
	}

	weeks := make([]int, 0, len(days))
	for w := range days {
		weeks = append(weeks, w)
	}
	sort.Ints(weeks)

	for _, w := range weeks {
		got := days[w]
		sort.Ints(got)
		for i, day := range got {
			if day != i+1 {
				t.Errorf("week %d has days %v — not a run starting at D1", w, got)
				break
			}
		}
	}

	// The weeks themselves must be a run too: a gap means a week of the plan
	// does not exist.
	for i, w := range weeks {
		if w != i+1 {
			t.Errorf("weeks are %v — expected an unbroken run from 1", weeks)
			break
		}
	}
}

// thinBrief is where a Do line stops carrying enough to work from. Tuned by
// running it: a first attempt also demanded a function name, a key press or a
// wikilink, and flagged 81 of 110 — an advisory that fires on three quarters of
// the plan is noise, and noise is worse than no check because it trains you to
// skip the output. Length alone turned out to be the signal that separates
// "insert a PivotTable from TeachingScheduleQuery, Ast to Rows" from "add a
// conditional column".
const thinBrief = 60

// Advisory, not a defect. On the hundred cases with no worked example the tutor
// improvises from Do alone, so the thinnest briefs are where it will drift
// furthest from what was intended — and therefore where a worked example or a
// longer Do buys the most.
func TestBriefsAreConcreteEnough(t *testing.T) {
	notes := vaultNotes(t)

	var thin []string
	for _, n := range notes {
		if do := fieldsOf(n)["Do"]; len(do) < thinBrief {
			thin = append(thin, fmt.Sprintf("%s(%d)", n.Slug, len(do)))
		}
	}
	if len(thin) > 0 {
		t.Logf("ADVISORY: %d of %d briefs are under %d characters — the tutor has "+
			"least to work with here:\n  %s",
			len(thin), len(notes), thinBrief, strings.Join(thin, " "))
	}
}
