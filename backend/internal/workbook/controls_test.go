package workbook

import (
	"bytes"
	"testing"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/vault"
	"github.com/xuri/excelize/v2"
)

// A control the student never sees is a requirement they cannot meet: the case
// says "type a code into E1 and the answer must follow", and if E1 is not in
// the file there is nothing to type into.
func TestControlsReachTheDataSheet(t *testing.T) {
	example := datasets.Example{
		InputColumns: []string{"Ast"},
		Controls: []datasets.Control{{
			Cell:  "E1",
			Label: "Search: assistant code",
			Value: "HI",
			Note:  "Change this and the lookup must follow.",
		}},
	}
	table := datasets.Table{
		Columns: []string{"Ast"},
		Rows:    []map[string]any{{"Ast": "AB"}, {"Ast": "HI"}},
	}

	raw, err := Build(vault.Note{Slug: "W1D5", Heading: "Lookups"}, example, &table)
	if err != nil {
		t.Fatalf("building: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer f.Close()

	if got, _ := f.GetCellValue(sheetData, "E1"); got != "HI" {
		t.Errorf("control value: got %q, want %q", got, "HI")
	}
	// The label goes one column to the left, so the sheet explains itself
	// without a legend.
	if got, _ := f.GetCellValue(sheetData, "D1"); got != "Search: assistant code" {
		t.Errorf("control label: got %q, want it in the cell left of the control", got)
	}
}

// Column A leaves nowhere for the label, so it is refused rather than silently
// dropping half the control.
func TestControlInColumnAIsRefused(t *testing.T) {
	example := datasets.Example{
		Controls: []datasets.Control{{Cell: "A1", Label: "x", Value: "y"}},
	}
	if _, err := Build(vault.Note{Slug: "W1D5"}, example, nil); err == nil {
		t.Fatal("expected an error for a control in column A, got none")
	}
}
