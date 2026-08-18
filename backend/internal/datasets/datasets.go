// Package datasets holds the two tables every case works against.
//
// The vault's notes name two real spreadsheets — TeachingScheduleQuery and the
// workshift Data sheet — that are not in the vault itself. These stand in for
// them, matching the shape the notes describe: 41 sessions over 21 assistants,
// a 40-person Mon-Sat roster, joined on Ast = Inisial.
//
// One dataset, reused by every case, is the point. Familiar data means a new
// case is a new *question*, not a new table to learn first — and the answers
// build on each other, which is what makes W9D2's join land after W5D1's sum.
package datasets

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed datasets.json
var raw []byte

// Table is one sheet: named columns and the rows under them.
//
// Rows are maps rather than slices so a column added to the JSON does not
// silently shift every value one place to the left. Columns carries the order,
// since a map has none.
type Table struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Columns     []string         `json:"columns"`
	Rows        []map[string]any `json:"rows"`
}

var (
	once    sync.Once
	tables  map[string]Table
	loadErr error
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(raw, &tables); err != nil {
			loadErr = fmt.Errorf("datasets.json: %w", err)
		}
	})
}

// All returns every dataset, keyed by id.
func All() (map[string]Table, error) {
	load()
	return tables, loadErr
}

// Get returns one dataset by id.
func Get(id string) (Table, bool) {
	load()
	t, ok := tables[id]
	return t, ok
}

// Head returns the first n rows of a table, for previewing a long sheet
// without shipping all 41 rows into a card that only needs to show the shape.
func (t Table) Head(n int) Table {
	if n >= len(t.Rows) {
		return t
	}
	out := t
	out.Rows = t.Rows[:n]
	return out
}
