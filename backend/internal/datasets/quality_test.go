package datasets

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Quality checks over the worked examples.
//
// These are the ten cases where the app states an answer as fact — shown on the
// page, written into the Expected sheet, and handed to the tutor as ground
// truth. A wrong one is not a blank space a student notices; it is a confident
// answer they trust and learn from.
//
// The check that matters most is the shape check below. W1D5 shipped with
// `=SORT(UNIQUE(tblTeaching[Ast]))` printed above a two-column expected result,
// and a one-column spill cannot fill two columns. Nothing caught it: the page
// rendered, the workbook built, the tutor was handed both. It took a human
// reading the case to notice.

func sorted(m map[string]Example) []string {
	out := make([]string, 0, len(m))
	for slug := range m {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

// spillFuncs return a single column. A formula built only from these cannot
// produce a result table wider than one column.
var spillFuncs = regexp.MustCompile(`^=\s*(SORT|UNIQUE|FILTER|SORTBY)\s*\(`)

// A formula and the answer printed beneath it have to be able to coexist. This
// is the W1D5 bug, encoded.
func TestFormulaShapeMatchesResultWidth(t *testing.T) {
	all := AllExamples()
	for _, slug := range sorted(all) {
		ex := all[slug]
		lines := strings.Split(strings.TrimSpace(ex.Formula), "\n")
		if len(lines) != 1 {
			continue // several formulas can between them fill several columns
		}
		if spillFuncs.MatchString(strings.TrimSpace(lines[0])) &&
			len(ex.Result.Columns) > 1 {
			t.Errorf("%s: formula %q spills one column but the expected result "+
				"claims %d: %v", slug, lines[0], len(ex.Result.Columns), ex.Result.Columns)
		}
	}
}

// Every column a case says it works from has to exist in the dataset it names.
// A typo here silently narrows the Data sheet to nothing.
func TestInputColumnsExist(t *testing.T) {
	all := AllExamples()
	for _, slug := range sorted(all) {
		ex := all[slug]
		table, ok := Get(ex.DatasetID)
		if !ok {
			t.Errorf("%s: names dataset %q, which does not exist", slug, ex.DatasetID)
			continue
		}
		have := map[string]bool{}
		for _, c := range table.Columns {
			have[c] = true
		}
		for _, c := range ex.InputColumns {
			if !have[c] {
				t.Errorf("%s: input column %q is not in dataset %q",
					slug, c, ex.DatasetID)
			}
		}
	}
}

// An expected result with no rows is a case with no answer — it renders as an
// empty table that reads like a failed load.
func TestExpectedResultsAreNotEmpty(t *testing.T) {
	all := AllExamples()
	for _, slug := range sorted(all) {
		ex := all[slug]
		if len(ex.Result.Columns) == 0 || len(ex.Result.Rows) == 0 {
			t.Errorf("%s: expected result is empty (%d columns, %d rows)",
				slug, len(ex.Result.Columns), len(ex.Result.Rows))
		}
	}
}

// Every row of the expected result must carry every column it claims. A missing
// key renders as a blank cell that looks like the answer is blank.
func TestExpectedResultRowsAreComplete(t *testing.T) {
	all := AllExamples()
	for _, slug := range sorted(all) {
		ex := all[slug]
		for i, row := range ex.Result.Rows {
			for _, col := range ex.Result.Columns {
				if _, ok := row[col]; !ok {
					t.Errorf("%s: result row %d has no %q", slug, i, col)
				}
			}
		}
	}
}

// functionName matches a Requires entry that names an Excel function, which is
// the only kind this check can verify by reading the formula.
var functionName = regexp.MustCompile(`^[A-Z][A-Z.]+$`)

// Requires names the techniques the answer must genuinely use. Where a case
// states them, the worked formula had better use them — otherwise the case is
// asking for one thing and demonstrating another, which is the defect that
// started this whole pass.
//
// Only function-shaped entries are enforced. Some requirements are real but not
// greppable — "structured references", "mixed references", "spill" — and the
// choice is between checking those badly or admitting the check does not cover
// them. Silently passing an entry nobody verified would be worse than either:
// it would read as proof where there is none, so the unenforceable ones are
// reported as exactly that.
func TestRequiredTechniquesAppearInTheFormula(t *testing.T) {
	all := AllExamples()
	var unenforceable []string

	for _, slug := range sorted(all) {
		ex := all[slug]
		upper := strings.ToUpper(ex.Formula)
		for _, req := range ex.Requires {
			if !functionName.MatchString(req) {
				unenforceable = append(unenforceable, slug+": "+req)
				continue
			}
			if !strings.Contains(upper, req) {
				t.Errorf("%s: requires %q but the worked formula never uses it:\n    %s",
					slug, req, strings.ReplaceAll(ex.Formula, "\n", "\n    "))
			}
		}
	}

	if len(unenforceable) > 0 {
		t.Logf("NOT VERIFIED (not function names — these rest on human review):\n  %s",
			strings.Join(unenforceable, "\n  "))
	}
}

// Advisory. A case with no Requires has nothing holding it to its objective,
// which is exactly how a student can satisfy the answer without ever touching
// the technique the case exists to teach.
func TestExamplesDeclareWhatTheyRequire(t *testing.T) {
	all := AllExamples()
	var bare []string
	for _, slug := range sorted(all) {
		if len(all[slug].Requires) == 0 {
			bare = append(bare, slug)
		}
	}
	if len(bare) > 0 {
		t.Logf("ADVISORY: %d of %d worked examples declare no required technique:\n  %s",
			len(bare), len(all), strings.Join(bare, " "))
	}
}

// The judge trusts these fields. A Computed column that is not in the result,
// or a SortedBy the stored rows do not obey, would make it fail a correct
// answer.
func TestJudgeFieldsAreConsistent(t *testing.T) {
	all := AllExamples()
	for _, slug := range sorted(all) {
		ex := all[slug]
		cols := map[string]bool{}
		for _, c := range ex.Result.Columns {
			cols[c] = true
		}

		if ex.WorkOn != "" && ex.WorkOn != "data" {
			t.Errorf("%s: workOn %q, want empty or \"data\"", slug, ex.WorkOn)
		}
		if ex.RowOrder != "" && ex.RowOrder != "listed" {
			t.Errorf("%s: rowOrder %q, want empty or \"listed\"", slug, ex.RowOrder)
		}
		for _, c := range ex.Checks {
			if c != "table" && c != "validation" {
				t.Errorf("%s: unknown check %q", slug, c)
			}
		}
		for _, c := range ex.Computed {
			if !cols[c] {
				t.Errorf("%s: computed column %q is not in the result", slug, c)
			}
		}

		for _, key := range ex.SortedBy {
			name := strings.TrimPrefix(key, "-")
			if !cols[name] {
				t.Errorf("%s: sortedBy %q is not in the result", slug, key)
			}
		}
		for i := 1; i < len(ex.Result.Rows); i++ {
			if compareByKeys(ex.Result.Rows[i-1], ex.Result.Rows[i], ex.SortedBy) > 0 {
				t.Errorf("%s: stored rows %d and %d break sortedBy %v",
					slug, i-1, i, ex.SortedBy)
			}
		}
	}
}

// compareByKeys orders two rows by SortedBy keys, numbers numerically.
func compareByKeys(a, b map[string]any, keys []string) int {
	for _, key := range keys {
		desc := strings.HasPrefix(key, "-")
		name := strings.TrimPrefix(key, "-")
		var c int
		af, aok := a[name].(float64)
		bf, bok := b[name].(float64)
		switch {
		case aok && bok && af < bf:
			c = -1
		case aok && bok && af > bf:
			c = 1
		case !aok || !bok:
			c = strings.Compare(norm(a[name]), norm(b[name]))
		}
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
	}
	return 0
}

// A case narrows the Data sheet to inputColumns, so that is all the student is
// given. A result column that is neither one of those nor derived from them is
// asking them to produce a value from data they cannot see.
//
// Advisory rather than a failure: a derived column ("Band", "Sessions") is the
// whole point of most cases, and no check can tell derived from missing. What
// it can do is list the ones worth a human glance.
func TestResultColumnsTraceToTheInputs(t *testing.T) {
	all := AllExamples()
	var unexplained []string

	for _, slug := range sorted(all) {
		ex := all[slug]
		given := map[string]bool{}
		for _, c := range ex.InputColumns {
			given[c] = true
		}
		for _, c := range ex.Result.Columns {
			if !given[c] {
				unexplained = append(unexplained, slug+": "+c)
			}
		}
	}
	if len(unexplained) > 0 {
		t.Logf("ADVISORY: result columns not among the case's input columns — "+
			"derived is fine, absent is not:\n  %s", strings.Join(unexplained, "\n  "))
	}
}
