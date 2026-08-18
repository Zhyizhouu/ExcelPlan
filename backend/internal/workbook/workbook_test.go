package workbook

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/vault"
)

func sampleNote() vault.Note {
	return vault.Note{
		Slug:    "W1D1",
		Title:   "W1D1 - Keyboard only",
		Heading: "W1D1 - Keyboard only",
		Phase:   "0",
		Week:    1,
		Fields: []vault.Field{
			{Label: "Objective", Value: "build shortcut muscle memory"},
			{Label: "Deliverable", Value: "the extract rebuilt, mouse untouched"},
		},
	}
}

func sampleExample() datasets.Example {
	return datasets.Example{
		DatasetID:    "teaching",
		InputColumns: []string{"Ast", "Workload"},
		Task:         "Retype the first rows without the mouse.",
		Formula:      "=SUMIFS(Workload,Ast,$A2)",
		ResultNote:   "Sessions must sum to 41.",
		Result: datasets.Result{
			Columns: []string{"Ast", "Total"},
			Rows: []map[string]any{
				{"Ast": "AB", "Total": 4.5},
				{"Ast": "BC", "Total": 3.0},
			},
		},
	}
}

func sampleTable() *datasets.Table {
	return &datasets.Table{
		ID:      "teaching",
		Name:    "TeachingScheduleQuery",
		Columns: []string{"Ast", "Workload"},
		Rows: []map[string]any{
			{"Ast": "AB", "Workload": 2.5},
			{"Ast": "BC", "Workload": 1.0},
		},
	}
}

func open(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("produced file is not a readable workbook: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestBuild(t *testing.T) {
	data, err := Build(sampleNote(), sampleExample(), sampleTable())
	if err != nil {
		t.Fatal(err)
	}
	f := open(t, data)

	t.Run("has exactly the three expected sheets", func(t *testing.T) {
		got := f.GetSheetList()
		want := []string{sheetBrief, sheetData, sheetExpected}
		if len(got) != len(want) {
			t.Fatalf("sheets = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("sheets = %v, want %v", got, want)
			}
		}
	})

	t.Run("data sheet has a header row and its rows", func(t *testing.T) {
		rows, err := f.GetRows(sheetData)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 3 { // header + 2
			t.Fatalf("got %d rows, want 3", len(rows))
		}
		if rows[0][0] != "Ast" || rows[0][1] != "Workload" {
			t.Fatalf("header = %v", rows[0])
		}
		if rows[1][0] != "AB" {
			t.Fatalf("first data row = %v", rows[1])
		}
	})

	// The answer lives on its own sheet so opening the file does not show it
	// beside the work. Losing that is a real regression in how the file teaches.
	t.Run("the expected result is on its own sheet, not beside the data", func(t *testing.T) {
		dataRows, _ := f.GetRows(sheetData)
		for _, row := range dataRows {
			for _, cell := range row {
				if cell == "Total" {
					t.Fatal("the expected result leaked onto the Data sheet")
				}
			}
		}
		expected, err := f.GetRows(sheetExpected)
		if err != nil || len(expected) != 3 {
			t.Fatalf("expected sheet rows = %d, err = %v", len(expected), err)
		}
	})

	// A cell starting with "=" is parsed as a formula and renders #NAME?, so
	// the one thing the learner is meant to read would be unreadable.
	t.Run("the formula is stored as text, not as a live formula", func(t *testing.T) {
		rows, err := f.GetRows(sheetBrief)
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, row := range rows {
			if len(row) > 1 && row[0] == "Formula" {
				found = true
				if !strings.Contains(row[1], "SUMIFS") {
					t.Fatalf("formula cell = %q", row[1])
				}
			}
		}
		if !found {
			t.Fatal("no Formula row on the Brief sheet")
		}
	})

	t.Run("the brief carries the note's own fields", func(t *testing.T) {
		rows, _ := f.GetRows(sheetBrief)
		var joined strings.Builder
		for _, row := range rows {
			joined.WriteString(strings.Join(row, " "))
			joined.WriteString("\n")
		}
		for _, want := range []string{"W1D1", "Objective", "muscle memory", "Deliverable"} {
			if !strings.Contains(joined.String(), want) {
				t.Fatalf("brief is missing %q", want)
			}
		}
	})
}

// A case with no worked example must still produce a valid file rather than
// an error — most of the 110 are in that state.
func TestBuildWithoutExample(t *testing.T) {
	data, err := Build(sampleNote(), datasets.Example{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := open(t, data)

	if len(f.GetSheetList()) != 3 {
		t.Fatalf("sheets = %v, want all three even when empty", f.GetSheetList())
	}
	rows, _ := f.GetRows(sheetData)
	if len(rows) == 0 || !strings.Contains(rows[0][0], "No data") {
		t.Fatalf("empty data sheet should explain itself, got %v", rows)
	}
}

func TestFilename(t *testing.T) {
	if got := Filename(sampleNote()); got != "W1D1 - Keyboard only - Answer.xlsx" {
		t.Fatalf("Filename = %q", got)
	}

	// The real vault has "W22D2 - Solver: balance the roster". A colon is
	// illegal in a Windows filename, so leaving it in turns a save into a
	// baffling OS error rather than a working file.
	t.Run("strips characters Windows refuses", func(t *testing.T) {
		note := vault.Note{Slug: "W22D2", Title: "W22D2 - Solver: balance the roster"}
		got := Filename(note)
		if strings.ContainsAny(got, `<>:"/\|?*`) {
			t.Fatalf("Filename kept an illegal character: %q", got)
		}
		if !strings.HasSuffix(got, " - Answer.xlsx") {
			t.Fatalf("Filename = %q", got)
		}
	})

	t.Run("falls back to the slug when there is no title", func(t *testing.T) {
		if got := Filename(vault.Note{Slug: "W9D9"}); got != "W9D9 - Answer.xlsx" {
			t.Fatalf("Filename = %q", got)
		}
	})
}
