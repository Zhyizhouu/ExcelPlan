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
	Result       Result   `json:"result"`
	ResultNote   string   `json:"resultNote"`
	Formula      string   `json:"formula"`
}

var (
	exOnce   sync.Once
	examples map[string]Example
)

// ExampleFor returns the worked example for a slug, if one has been written.
//
// Most of the 110 cases do not have one yet. The second return value lets the
// UI say so plainly instead of rendering an empty table that reads like a bug.
func ExampleFor(slug string) (Example, bool) {
	exOnce.Do(func() {
		_ = json.Unmarshal(rawExamples, &examples)
	})
	ex, ok := examples[slug]
	return ex, ok
}

// HaveExamples reports how many cases carry a worked example, so the UI can be
// honest about coverage rather than implying every case has one.
func HaveExamples() int {
	exOnce.Do(func() {
		_ = json.Unmarshal(rawExamples, &examples)
	})
	return len(examples)
}
