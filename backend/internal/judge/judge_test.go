package judge

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/workbook"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// book builds a workbook with the four sheets the app writes.
func book(t *testing.T, fill func(f *excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	must(t, f.SetSheetName("Sheet1", workbook.SheetBrief))
	for _, s := range []string{workbook.SheetData, workbook.SheetAnswer, workbook.SheetExpected} {
		_, err := f.NewSheet(s)
		must(t, err)
	}
	fill(f)
	var buf bytes.Buffer
	must(t, f.Write(&buf))
	return buf.Bytes()
}

// formula writes a cell the way Excel saves one: a formula with its value.
func formula(t *testing.T, f *excelize.File, cell, expr string, value any) {
	t.Helper()
	must(t, f.SetCellValue(workbook.SheetAnswer, cell, value))
	must(t, f.SetCellFormula(workbook.SheetAnswer, cell, expr))
}

func summary() datasets.Example {
	return datasets.Example{
		Requires: []string{"SUMIF"},
		Computed: []string{"Total Workload"},
		Result: datasets.Result{
			Columns: []string{"Ast", "Total Workload"},
			Rows: []map[string]any{
				{"Ast": "AB", "Total Workload": 5.0},
				{"Ast": "HI", "Total Workload": 5.0},
				{"Ast": "CD", "Total Workload": 2.0},
			},
		},
	}
}

// answer writes the summary at top-left, with Total Workload as formulas.
func answer(t *testing.T, f *excelize.File, top string, rows [][]any, fn string) {
	t.Helper()
	col, row, err := excelize.CellNameToCoordinates(top)
	must(t, err)
	cell := func(c, r int) string { n, _ := excelize.CoordinatesToCellName(c, r); return n }
	must(t, f.SetCellValue(workbook.SheetAnswer, cell(col, row), "Ast"))
	must(t, f.SetCellValue(workbook.SheetAnswer, cell(col+1, row), "Total Workload"))
	for i, r := range rows {
		must(t, f.SetCellValue(workbook.SheetAnswer, cell(col, row+1+i), r[0]))
		formula(t, f, cell(col+1, row+1+i), fn+"(Data!A:A,"+cell(col, row+1+i)+",Data!B:B)", r[1])
	}
}

func run(t *testing.T, data []byte, ex datasets.Example) Report {
	t.Helper()
	r, err := Judge(data, ex)
	must(t, err)
	return r
}

func find(r Report, id string) Check {
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	return Check{}
}

func mentions(c Check, text string) bool {
	return strings.Contains(c.Detail+"\n"+strings.Join(c.Items, "\n"), text)
}

var good = [][]any{{"AB", 5}, {"HI", 5}, {"CD", 2}}

func TestFindsTheAnswerAnywhereAndPasses(t *testing.T) {
	data := book(t, func(f *excelize.File) { answer(t, f, "D3", good, "SUMIF") })
	r := run(t, data, summary())
	if !r.Passed {
		t.Fatalf("want a pass, got %+v", r.Checks)
	}
	if c := find(r, "found"); !mentions(c, "Answer!D3:E6") {
		t.Fatalf("found = %+v", c)
	}
}

func TestWrongValueNamesTheRowAndColumn(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		answer(t, f, "A1", [][]any{{"AB", 5}, {"HI", 5}, {"CD", 3.5}}, "SUMIF")
	})
	c := find(run(t, data, summary()), "values")
	if c.Status != Fail || !mentions(c, "Ast CD: Total Workload is 3.5, expected 2.") {
		t.Fatalf("values = %+v", c)
	}
}

func TestTypedValuesAreCaught(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		must(t, f.SetSheetRow(workbook.SheetAnswer, "A1", &[]any{"Ast", "Total Workload"}))
		for i, r := range good {
			cell, _ := excelize.CoordinatesToCellName(1, i+2)
			must(t, f.SetSheetRow(workbook.SheetAnswer, cell, &r))
		}
	})
	r := run(t, data, summary())
	if find(r, "values").Status != Pass {
		t.Fatalf("typed values should still match: %+v", find(r, "values"))
	}
	if c := find(r, "computed"); c.Status != Fail || !mentions(c, "Total Workload: 3 of 3") {
		t.Fatalf("computed = %+v", c)
	}
	if r.Passed {
		t.Fatal("a typed answer must not pass")
	}
}

func TestMissingHeaderSaysWhich(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		must(t, f.SetSheetRow(workbook.SheetAnswer, "A1", &[]any{"Ast", "Total"}))
	})
	c := find(run(t, data, summary()), "found")
	if c.Status != Fail || !mentions(c, "has Ast but not Total Workload") {
		t.Fatalf("found = %+v", c)
	}
}

func TestTwoTablesAreAmbiguous(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		answer(t, f, "A1", good, "SUMIF")
		answer(t, f, "A10", good, "SUMIF")
	})
	c := find(run(t, data, summary()), "found")
	if c.Status != Fail || len(c.Items) != 2 {
		t.Fatalf("found = %+v", c)
	}
}

func TestSumifsDoesNotCountAsSumif(t *testing.T) {
	data := book(t, func(f *excelize.File) { answer(t, f, "A1", good, "SUMIFS") })
	c := find(run(t, data, summary()), "techniques")
	if c.Status != Fail || !mentions(c, "uses SUMIF.") {
		t.Fatalf("techniques = %+v", c)
	}
}

func TestSortAllowsTiesButNotDisorder(t *testing.T) {
	ex := summary()
	ex.SortedBy = []string{"-Total Workload"}

	tied := book(t, func(f *excelize.File) {
		answer(t, f, "A1", [][]any{{"HI", 5}, {"AB", 5}, {"CD", 2}}, "SUMIF")
	})
	if c := find(run(t, tied, ex), "order"); c.Status != Pass {
		t.Fatalf("ties in either order should pass: %+v", c)
	}

	wrong := book(t, func(f *excelize.File) {
		answer(t, f, "A1", [][]any{{"AB", 5}, {"CD", 2}, {"HI", 5}}, "SUMIF")
	})
	if c := find(run(t, wrong, ex), "order"); c.Status != Fail {
		t.Fatalf("out of order should fail: %+v", c)
	}
}

func TestSampleAllowsExtraRows(t *testing.T) {
	rows := append(append([][]any{}, good...), []any{"ZZ", 1})
	data := book(t, func(f *excelize.File) { answer(t, f, "A1", rows, "SUMIF") })

	ex := summary()
	if c := find(run(t, data, ex), "values"); !mentions(c, "Extra row: Ast ZZ.") {
		t.Fatalf("a full result should flag the extra row: %+v", c)
	}
	ex.Sample = true
	if c := find(run(t, data, ex), "values"); c.Status != Pass {
		t.Fatalf("a sample should allow extra rows: %+v", c)
	}
}

func TestHiddenRowsAreSkipped(t *testing.T) {
	rows := append(append([][]any{}, good...), []any{"ZZ", 1})
	data := book(t, func(f *excelize.File) {
		answer(t, f, "A1", rows, "SUMIF")
		must(t, f.SetRowVisible(workbook.SheetAnswer, 5, false))
	})
	if c := find(run(t, data, summary()), "values"); c.Status != Pass {
		t.Fatalf("a filtered-out row should not count: %+v", c)
	}
}

func TestOldWorkbookWithoutAnswerSheet(t *testing.T) {
	data := book(t, func(f *excelize.File) { must(t, f.DeleteSheet(workbook.SheetAnswer)) })
	r := run(t, data, summary())
	if c := find(r, "sheet"); c.Status != Fail || !mentions(c, "before the Answer sheet existed") {
		t.Fatalf("sheet = %+v", c)
	}
}

func TestValidationRules(t *testing.T) {
	ex := datasets.Example{
		WorkOn: "data",
		Checks: []string{"validation"},
		Result: datasets.Result{
			Columns: []string{"Field", "Rule", "Allowed"},
			Rows: []map[string]any{
				{"Field": "Group", "Rule": "List", "Allowed": "G1, G2"},
				{"Field": "Lecturer Type", "Rule": "List", "Allowed": "VC, Forum"},
				{"Field": "Workload", "Rule": "Decimal between", "Allowed": "0 to 1"},
			},
		},
	}
	data := book(t, func(f *excelize.File) {
		must(t, f.SetSheetRow(workbook.SheetData, "A1", &[]any{"Group", "Lecturer Type", "Workload"}))
		list := excelize.NewDataValidation(true)
		list.Sqref = "A2:A42"
		must(t, list.SetDropList([]string{"G1", "G2"}))
		must(t, f.AddDataValidation(workbook.SheetData, list))
		dec := excelize.NewDataValidation(true)
		dec.Sqref = "C2:C42"
		must(t, dec.SetRange(0, 1, excelize.DataValidationTypeDecimal, excelize.DataValidationOperatorBetween))
		must(t, f.AddDataValidation(workbook.SheetData, dec))
	})
	c := find(run(t, data, ex), "validation")
	if c.Status != Fail || len(c.Items) != 1 || !mentions(c, "Lecturer Type: no validation rule.") {
		t.Fatalf("validation = %+v", c)
	}
}

func TestCloseEnough(t *testing.T) {
	cases := []struct {
		got, want float64
		ok        bool
	}{
		{1.7777, 1.78, true},
		{0.21951219, 0.2195, true},
		{4.4, 4, false},
		{4.5, 4.5, true},
		{14.499999999999998, 14.5, true},
		{1.79, 1.78, false},
	}
	for _, c := range cases {
		if closeEnough(c.got, c.want) != c.ok {
			t.Errorf("closeEnough(%v, %v) = %v", c.got, c.want, !c.ok)
		}
	}
}

// Excel stores a spill as one array formula with a ref; the cells it fills
// hold only values. This is hand-written XML in that shape.
func TestScanFormulasReadsSpillRanges(t *testing.T) {
	parts := map[string]string{
		"xl/workbook.xml": `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Answer" sheetId="2" r:id="rId7"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships>` +
			`<Relationship Id="rId1" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId7" Target="/xl/worksheets/sheet9.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData/></worksheet>`,
		"xl/worksheets/sheet9.xml": `<worksheet><sheetData>` +
			`<row r="2"><c r="D2" cm="1"><f t="array" ref="D2:E4">_xlfn._xlws.SORT(Data!A2:B4)</f><v>AB</v></c><c r="E2"><v>5</v></c></row>` +
			`<row r="3"><c r="D3" t="str"><v>HI</v></c><c r="F3"><f t="shared" ref="F3:F4" si="0">D3&amp;E3</f><v>HI5</v></c></row>` +
			`<row r="4"><c r="F4"><f t="shared" si="0"/><v>x</v></c><c r="G4"><v>1</v></c></row>` +
			`</sheetData></worksheet>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range parts {
		w, err := zw.Create(name)
		must(t, err)
		_, err = w.Write([]byte(body))
		must(t, err)
	}
	must(t, zw.Close())

	scan, err := scanFormulas(buf.Bytes())
	must(t, err)
	sf, ok := scan["Answer"]
	if !ok {
		t.Fatalf("Answer sheet not found through its relationship: %v", scan)
	}
	for _, cell := range []string{"D2", "E3", "D4", "E4", "F3", "F4"} {
		col, row, _ := excelize.CellNameToCoordinates(cell)
		if !sf.computed(col, row) {
			t.Errorf("%s should count as computed", cell)
		}
	}
	for _, cell := range []string{"G4", "D5"} {
		col, row, _ := excelize.CellNameToCoordinates(cell)
		if sf.computed(col, row) {
			t.Errorf("%s should not count as computed", cell)
		}
	}
	if !usesFunction(sf.texts, "SORT") {
		t.Errorf("SORT behind its _xlfn._xlws. prefix was not recognised: %v", sf.texts)
	}
}
