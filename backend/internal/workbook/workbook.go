// Package workbook builds the .xlsx a case is worked in.
//
// Four sheets: the brief, the data, a blank Answer sheet, and the expected
// result. The student builds their own table on Answer, which is where the
// judge looks for it. The expected result gets its own sheet rather than
// sitting beside the data, so opening the file does not hand over the answer
// before the work starts.
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
	sheetAnswer   = "Answer"
	sheetExpected = "Expected"
)

// Sheet names the judge reads.
const (
	SheetBrief    = sheetBrief
	SheetData     = sheetData
	SheetAnswer   = sheetAnswer
	SheetExpected = sheetExpected
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
	if err := writeControls(f, example, label); err != nil {
		return nil, err
	}
	// Deliberately blank: the layout is part of the work.
	if _, err := f.NewSheet(sheetAnswer); err != nil {
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
	if len(example.Result.Columns) > 0 {
		rows = append(rows, [2]string{"Work on", workOn(example)})
		// The exact names the judge searches for, so nobody has to open
		// Expected to learn what to call a column.
		if example.Judged() {
			rows = append(rows, [2]string{"Headers", strings.Join(example.Result.Columns, ", ")})
		}
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

// HasAnswerSheet reports whether a saved workbook was built with the Answer
// sheet. Only workbooks made before it existed may be replaced.
func HasAnswerSheet(data []byte) (bool, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	defer f.Close()
	idx, err := f.GetSheetIndex(sheetAnswer)
	return err == nil && idx >= 0, nil
}

func workOn(example datasets.Example) string {
	switch {
	case example.WorkOn == "data":
		return "The Data sheet itself. The checker reads your work there."
	case !example.Judged():
		return "The Answer sheet. This case's example is a reference, so compare " +
			"your work with the Expected sheet yourself."
	}
	return "The Answer sheet. Build your own table there with the headers below, " +
		"anywhere on the sheet, and save before checking."
}

// writeControls puts the case's input cells on the Data sheet.
//
// Beside the data rather than on the Brief, because the whole point is that the
// answer reacts to them: a control the student has to switch tabs to change is
// one they will forget is driving anything. The label goes in the cell to the
// control's left, so the sheet reads without a legend.
//
// Values are written as text, including ones that look numeric. A control is
// something typed over, and Excel silently reformatting the cell the first time
// it is edited is a distraction from the case.
func writeControls(f *excelize.File, example datasets.Example, label int) error {
	for _, c := range example.Controls {
		col, row, err := excelize.CellNameToCoordinates(c.Cell)
		if err != nil {
			return fmt.Errorf("control cell %q: %w", c.Cell, err)
		}
		if col < 2 {
			// The label would land in column zero. Caught here rather than
			// silently dropping the label.
			return fmt.Errorf("control cell %q must be in column B or later", c.Cell)
		}

		labelCell, err := excelize.CoordinatesToCellName(col-1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellStr(sheetData, labelCell, c.Label); err != nil {
			return err
		}
		if err := f.SetCellStr(sheetData, c.Cell, c.Value); err != nil {
			return err
		}
		_ = f.SetCellStyle(sheetData, labelCell, labelCell, label)

		if c.Note != "" {
			// A comment rather than another cell: the note explains the control
			// without competing with it for space next to the data.
			_ = f.AddComment(sheetData, excelize.Comment{
				Cell:   c.Cell,
				Author: "ExcelPlan",
				Paragraph: []excelize.RichTextRun{
					{Text: c.Note},
				},
			})
		}
	}
	return nil
}

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
