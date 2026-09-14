package datasets

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

// A control cell that lands inside the data block silently overwrites a value,
// and the case then teaches against corrupted input. The collision is invisible
// in every other check — the workbook builds, the sheet opens, one cell is
// simply wrong — so it is asserted here rather than left to be noticed.
func TestControlCellsClearOfTheData(t *testing.T) {
	for slug, example := range AllExamples() {
		for _, control := range example.Controls {
			col, row, err := excelize.CellNameToCoordinates(control.Cell)
			if err != nil {
				t.Errorf("%s: control cell %q is not a cell reference: %v",
					slug, control.Cell, err)
				continue
			}
			if col < 2 {
				t.Errorf("%s: control cell %q leaves no room for its label to the left",
					slug, control.Cell)
			}

			table, ok := Get(example.DatasetID)
			if !ok {
				continue
			}
			// The Data sheet holds only the columns the case narrows to, from
			// A1, with one header row. The label sits one column left of the
			// control, so that column has to be clear too.
			width := len(example.InputColumns)
			if width == 0 {
				width = len(table.Columns)
			}
			height := len(table.Rows) + 1

			if col-1 <= width && row <= height {
				t.Errorf("%s: control %q (label at column %d) overlaps the data block "+
					"of %d columns x %d rows", slug, control.Cell, col-1, width, height)
			}
		}
	}
}
