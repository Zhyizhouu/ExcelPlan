// Package workbook builds the .xlsx a case is worked in.
//
// Three sheets: the brief, the data, and the expected result. The expected
// result gets its own sheet rather than sitting beside the data, so opening
// the file does not hand over the answer before the work starts — it is there
// to check against, one tab away, which is a different thing from being shown.
//
// The Data sheet is a plain styled range and deliberately *not* an Excel
// Table. W2D1's entire exercise is converting a range to a Table with Ctrl+T;
// shipping one pre-made would complete that case on the learner's behalf.
package workbook

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/vault"
)

const (
	sheetBrief    = "Brief"
	sheetData     = "Data"
	sheetExpected = "Expected"
)

// Build renders a case into a workbook.
//
// example and table may be zero-valued: a case with no worked example still
// gets a usable file with its brief in it, rather than an error.
func Build(note vault.Note, example datasets.Example, table *datasets.Table) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// NewFile starts with "Sheet1"; rename it rather than creating a fourth
	// and leaving an empty one behind.
	if err := f.SetSheetName("Sheet1", sheetBrief); err != nil {
		return nil, err
	}

	header, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "1C2620"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EEE7"}},
		Border: []excelize.Border{
			{Type: "bottom", Color: "2D6A4F", Style: 2},
		},
	})
	if err != nil {
		return nil, err
	}
	label, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "646A62"},
		Alignment: &excelize.Alignment{Vertical: "top"},
	})
	if err != nil {
		return nil, err
	}
	wrap, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	if err != nil {
		return nil, err
	}

	if err := writeBrief(f, note, example, label, wrap); err != nil {
		return nil, err
	}

	// A case with no worked example still gets all three sheets, the data ones
	// carrying a note saying why they are empty. A workbook missing a tab it
	// normally has reads as a broken export.
	var cols []string
	var rows []map[string]any
	if table != nil {
		cols, rows = table.Columns, table.Rows
	}
	if err := writeTable(f, sheetData, cols, rows, header); err != nil {
		return nil, err
	}
	if err := writeTable(f, sheetExpected,
		example.Result.Columns, example.Result.Rows, header); err != nil {
		return nil, err
	}

	// Open on the Brief, so the file explains itself before it shows data.
	if idx, err := f.GetSheetIndex(sheetBrief); err == nil {
		f.SetActiveSheet(idx)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("writing workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func writeBrief(
	f *excelize.File, note vault.Note, example datasets.Example, label, wrap int,
) error {
	title := strings.TrimSpace(note.Heading)
	if title == "" {
		title = note.Title
	}

	rows := [][2]string{
		{"Case", note.Slug},
		{"Title", title},
		{"Phase", note.Phase},
		{"Week", fmt.Sprintf("%d", note.Week)},
		{"", ""},
	}
	for _, field := range note.Fields {
		rows = append(rows, [2]string{field.Label, field.Value})
	}
	if example.Task != "" {
		rows = append(rows, [2]string{"", ""}, [2]string{"Task", example.Task})
	}
	if example.Formula != "" {
		// Prefixed with an apostrophe so Excel stores it as text. Without it a
		// cell beginning "=" is parsed as a formula and shows #NAME? instead of
		// the thing the learner is supposed to read.
		rows = append(rows, [2]string{"Formula", "'" + example.Formula})
	}
	if example.ResultNote != "" {
		rows = append(rows, [2]string{"Check", example.ResultNote})
	}

	for i, row := range rows {
		n := i + 1
		if err := f.SetCellValue(sheetBrief, cell("A", n), row[0]); err != nil {
			return err
		}
		if err := f.SetCellValue(sheetBrief, cell("B", n), row[1]); err != nil {
			return err
		}
	}

	_ = f.SetColWidth(sheetBrief, "A", "A", 14)
	_ = f.SetColWidth(sheetBrief, "B", "B", 96)
	_ = f.SetCellStyle(sheetBrief, "A1", cell("A", len(rows)), label)
	_ = f.SetCellStyle(sheetBrief, "B1", cell("B", len(rows)), wrap)
	return nil
}

// writeTable lays a table out with a styled, frozen header row.
func writeTable(
	f *excelize.File, sheet string, columns []string, rows []map[string]any, header int,
) error {
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}
	if len(columns) == 0 {
		return f.SetCellValue(sheet, "A1", "No data for this case yet.")
	}

	for i, col := range columns {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell(name, 1), col); err != nil {
			return err
		}
		_ = f.SetColWidth(sheet, name, name, widthFor(col, rows))
	}

	for r, row := range rows {
		for i, col := range columns {
			name, err := excelize.ColumnNumberToName(i + 1)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell(name, r+2), row[col]); err != nil {
				return err
			}
		}
	}

	last, err := excelize.ColumnNumberToName(len(columns))
	if err != nil {
		return err
	}
	_ = f.SetCellStyle(sheet, "A1", cell(last, 1), header)
	// Freeze the header so scrolling 41 rows does not lose the column names —
	// the first thing anybody does to a sheet this size by hand.
	_ = f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: 1,
		TopLeftCell: "A2", ActivePane: "bottomLeft",
	})
	return nil
}

// widthFor sizes a column to its widest value, within reason.
func widthFor(column string, rows []map[string]any) float64 {
	width := len(column)
	for _, row := range rows {
		if n := len(fmt.Sprintf("%v", row[column])); n > width {
			width = n
		}
	}
	return min(float64(width)+3, 46)
}

func cell(col string, row int) string { return fmt.Sprintf("%s%d", col, row) }

// Filename is the workbook's name, both for the download and for the copy
// saved into the vault: "W1D1 - Keyboard only - Answer.xlsx".
//
// Built from the note's own title so it sits alphabetically beside its case in
// the phase folder. The .xlsx extension is also what keeps it invisible to the
// note parser, which only ever globs *.md.
func Filename(note vault.Note) string {
	base := strings.TrimSpace(note.Title)
	if base == "" {
		base = note.Slug
	}
	return sanitise(base) + " - Answer.xlsx"
}

// sanitise strips the characters Windows refuses in a filename.
//
// Titles come from frontmatter a human typed, and one of them really does
// contain a colon ("W22D2 - Solver: balance the roster"), which would make the
// write fail with a baffling error rather than an obvious one.
func sanitise(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, s)
}
