package datasets

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed examples.json
var rawExamples []byte

// Result is a small table: the answer a case should produce.
type Result struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

// Control is an input cell the answer has to react to.
//
// The point of one is to make cheating visible. A case that says "classify
// each session" can be satisfied by typing the labels in by hand, and the
// finished sheet looks identical to one built with the formula the case was
// teaching — so the student skips the technique, gets the right answer, and
// learns nothing. Put the threshold in a cell instead, and a hand-typed column
// stops being right the moment that cell changes.
//
// Label is written into the cell immediately to the left of Cell, so the sheet
// explains itself without a legend.
type Control struct {
	// Where the value goes, e.g. "E1". Must sit clear of the data block —
	// there is a test that fails if it does not.
	Cell  string `json:"cell"`
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note"`
}

// Example is one case's worked demonstration — what to look at, what to do,
// and what should come out.
//
// Every Result in examples.json is *computed* from the dataset rather than
// typed, because a worked answer that disagrees with the data it claims to
// come from teaches the wrong thing with full confidence.
type Example struct {
	DatasetID    string   `json:"dataset"`
	InputColumns []string `json:"inputColumns"`
	InputNote    string   `json:"inputNote"`
	Task         string   `json:"task"`
	// Requires names the techniques the answer must genuinely use.
	//
	// Stated rather than implied, because the note's objective and the task did
	// not always agree: W1D5's objective named XLOOKUP while nothing in its task
	// needed a lookup at all. Naming them here puts the claim next to the work,
	// and gives the tutor something to hold the student to.
	Requires []string `json:"requires,omitempty"`
	// Controls are the input cells that make the requirement bite.
	Controls   []Control `json:"controls,omitempty"`
	Result     Result    `json:"result"`
	ResultNote string    `json:"resultNote"`
	Formula    string    `json:"formula"`

	// The fields below tell the judge how to read an answer.

	// WorkOn is "data" when the case is done on the Data sheet itself (a Table,
	// a sort, a split in place); empty means the student builds on Answer.
	WorkOn string `json:"workOn,omitempty"`
	// Computed lists result columns that must come from formulas, not typing.
	Computed []string `json:"computed,omitempty"`
	// SortedBy is the order rows must be in; a "-" prefix means descending.
	// Ties may come in any order.
	SortedBy []string `json:"sortedBy,omitempty"`
	// RowOrder "listed" means rows must appear exactly as listed, for orders
	// no sort key expresses, like Monday to Saturday.
	RowOrder string `json:"rowOrder,omitempty"`
	// Sample marks a result that shows some rows of the answer, not all.
	Sample bool `json:"sample,omitempty"`
	// Checks names non-value checks: "table" (a Table exists on the Data
	// sheet) and "validation" (Result rows describe validation rules, not a
	// table of values).
	Checks []string `json:"checks,omitempty"`
	// NotChecked names parts of the case the judge cannot see, like a chart.
	NotChecked []string `json:"notChecked,omitempty"`
	// Reference marks an example shown for study only: its answer is a pivot,
	// a chart or a written judgement the checker cannot compare.
	Reference bool `json:"reference,omitempty"`
	// Practice marks a warm-up case the user chose not to have checked.
	Practice bool `json:"practice,omitempty"`
}

// Judged reports whether the answer is a table of values the judge compares,
// as opposed to rules it inspects.
func (e Example) Judged() bool {
	if e.Reference || e.Practice {
		return false
	}
	for _, c := range e.Checks {
		if c == "validation" {
			return false
		}
	}
	return len(e.Result.Columns) > 0
}

var (
	exOnce   sync.Once
	examples map[string]Example
)

// ExampleFor returns the worked example for a slug, if one has been written.
//
// Most of the 127 cases do not have one yet. The second return value lets the
// UI say so plainly instead of rendering an empty table that reads like a bug.
func ExampleFor(slug string) (Example, bool) {
	exOnce.Do(func() {
		_ = json.Unmarshal(rawExamples, &examples)
	})
	ex, ok := examples[slug]
	return ex, ok
}

// AllExamples returns every worked example, keyed by slug.
//
// For checks that have to hold across the whole set rather than one case at a
// time — a control cell landing on top of the data, say, which is invisible in
// any single-case test because the workbook still builds and still opens.
func AllExamples() map[string]Example {
	exOnce.Do(func() {
		_ = json.Unmarshal(rawExamples, &examples)
	})
	out := make(map[string]Example, len(examples))
	for slug, ex := range examples {
		out[slug] = ex
	}
	return out
}

// HaveExamples reports how many cases carry a worked example, so the UI can be
// honest about coverage rather than implying every case has one.
func HaveExamples() int {
	exOnce.Do(func() {
		_ = json.Unmarshal(rawExamples, &examples)
	})
	return len(examples)
}
