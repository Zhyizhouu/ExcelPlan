// Package judge checks a saved answer workbook against a case's worked
// example. Every verdict comes from plain comparisons, never a model, so the
// same file always gets the same result.
package judge

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
	"github.com/Zhyizhouu/excelplan/internal/workbook"
)

type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
)

// Check is one line of the report.
type Check struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Status Status   `json:"status"`
	Detail string   `json:"detail,omitempty"`
	Items  []string `json:"items,omitempty"`
}

// Report is the verdict on one workbook.
type Report struct {
	Passed bool    `json:"passed"`
	Checks []Check `json:"checks"`
	// NotChecked names what the judge cannot see, so a pass is not read as
	// covering it.
	NotChecked []string `json:"notChecked,omitempty"`
}

const maxItems = 8

// Judge reads an .xlsx and checks it against the example.
func Judge(data []byte, ex datasets.Example) (Report, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Report{}, fmt.Errorf("opening workbook: %w", err)
	}
	defer f.Close()

	formulas, err := scanFormulas(data)
	if err != nil {
		return Report{}, fmt.Errorf("reading formulas: %w", err)
	}

	j := &judging{f: f, ex: ex, formulas: formulas}
	j.run()
	return j.report(), nil
}

type judging struct {
	f          *excelize.File
	ex         datasets.Example
	formulas   map[string]sheetFormulas
	checks     []Check
	notChecked []string
}

func (j *judging) run() {
	sheet := workbook.SheetAnswer
	// A Data-sheet case may be done on a copy on Answer instead; use it when it is there.
	if j.ex.WorkOn == "data" && !j.hasHeaders(workbook.SheetAnswer) {
		sheet = workbook.SheetData
	}
	if idx, err := j.f.GetSheetIndex(sheet); err != nil || idx < 0 {
		j.add(Check{ID: "sheet", Label: "The " + sheet + " sheet is there", Status: Fail,
			Detail: missingSheet(sheet)})
		j.explainUnchecked()
		return
	}

	if j.ex.Judged() {
		if b := j.findBlock(sheet); b != nil {
			j.checkValues(b)
			j.checkOrder(b)
			j.checkComputed(b)
		}
	}
	j.checkTechniques()
	if slices.Contains(j.ex.Checks, "table") {
		j.checkTable()
	}
	if slices.Contains(j.ex.Checks, "validation") {
		j.checkValidation()
	}
	j.explainUnchecked()
}

func missingSheet(sheet string) string {
	if sheet == workbook.SheetAnswer {
		return "This workbook was made before the Answer sheet existed. Use Replace with " +
			"new template on the case page to get a fresh copy."
	}
	return "The " + sheet + " sheet has been renamed or deleted."
}

func (j *judging) add(c Check) {
	if len(c.Items) > maxItems {
		more := len(c.Items) - maxItems
		c.Items = append(c.Items[:maxItems:maxItems], fmt.Sprintf("…and %d more.", more))
	}
	j.checks = append(j.checks, c)
}

func (j *judging) report() Report {
	passed := len(j.checks) > 0
	for _, c := range j.checks {
		if c.Status != Pass {
			passed = false
		}
	}
	return Report{Passed: passed, Checks: j.checks, NotChecked: j.notChecked}
}

func (j *judging) explainUnchecked() {
	if n := len(j.ex.Controls); n > 0 {
		cells := make([]string, n)
		for i, c := range j.ex.Controls {
			cells[i] = c.Cell
		}
		noun := "the control cell "
		if n > 1 {
			noun = "the control cells "
		}
		j.notChecked = append(j.notChecked,
			"Whether your answer follows "+noun+joinAnd(cells)+" when you change it. Change it and watch.")
	}
	j.notChecked = append(j.notChecked, j.ex.NotChecked...)
}

// --- finding the answer ---------------------------------------------------

type block struct {
	sheet  string
	header int            // header row number
	col    map[string]int // expected column name to column number
	rows   []int          // visible data rows, top to bottom
	all    []int          // every data row, hidden ones included
	grid   [][]string
}

// hasHeaders reports whether some row on sheet holds every expected header.
func (j *judging) hasHeaders(sheet string) bool {
	cols := j.ex.Result.Columns
	grid, err := j.f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil || len(cols) == 0 {
		return false
	}
	for _, row := range grid {
		have := map[string]bool{}
		for _, v := range row {
			have[normHeader(v)] = true
		}
		if !slices.ContainsFunc(cols, func(c string) bool { return !have[normHeader(c)] }) {
			return true
		}
	}
	return false
}

func (b *block) value(row int, column string) string {
	c, ok := b.col[column]
	if !ok || row-1 >= len(b.grid) || c-1 >= len(b.grid[row-1]) {
		return ""
	}
	return b.grid[row-1][c-1]
}

func (b *block) where() string {
	first, last := math.MaxInt, 0
	for _, c := range b.col {
		first, last = min(first, c), max(last, c)
	}
	bottom := b.header
	if len(b.rows) > 0 {
		bottom = b.rows[len(b.rows)-1]
	}
	from, _ := excelize.CoordinatesToCellName(first, b.header)
	to, _ := excelize.CoordinatesToCellName(last, bottom)
	return b.sheet + "!" + from + ":" + to
}

func normHeader(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// findBlock looks for the one row holding every expected header, anywhere on
// the sheet, and reads the rows under it down to the first blank one.
func (j *judging) findBlock(sheet string) *block {
	check := Check{ID: "found", Label: "Found your answer table"}
	cols := j.ex.Result.Columns

	grid, err := j.f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		check.Status, check.Detail = Fail, "Could not read the "+sheet+" sheet."
		j.add(check)
		return nil
	}

	var found []*block
	var twice []int
	bestRow, bestHave := 0, []string(nil)
	for i, row := range grid {
		at := map[string][]int{}
		for c, v := range row {
			if k := normHeader(v); k != "" {
				at[k] = append(at[k], c+1)
			}
		}

		var have []string
		for _, name := range cols {
			if len(at[normHeader(name)]) > 0 {
				have = append(have, name)
			}
		}
		if len(have) < len(cols) {
			if len(have) > len(bestHave) {
				bestRow, bestHave = i+1, have
			}
			continue
		}
		// Every header twice means two whole tables side by side. One header
		// twice is usually a summary sharing a row with its source table.
		if !slices.ContainsFunc(cols, func(c string) bool { return len(at[normHeader(c)]) < 2 }) {
			twice = append(twice, i+1)
			continue
		}
		b := &block{sheet: sheet, header: i + 1, col: closestHeaders(cols, at), grid: grid}
		b.rows, b.all = j.dataRows(b)
		found = append(found, b)
	}

	switch {
	case len(twice) > 0:
		check.Status = Fail
		check.Detail = fmt.Sprintf("Row %d has the headers twice. Keep one table, or rename "+
			"the headers on the other.", twice[0])
	case len(found) > 1:
		check.Status = Fail
		check.Detail = "More than one table has these headers. Keep one, or rename the " +
			"headers on the others."
		for _, b := range found {
			check.Items = append(check.Items, b.where())
		}
	case len(found) == 0 && len(bestHave) > 0:
		check.Status = Fail
		check.Detail = fmt.Sprintf("Row %d has %s but not %s. The headers must match the Brief.",
			bestRow, joinAnd(bestHave), joinAnd(missingFrom(cols, bestHave)))
	case len(found) == 0:
		check.Status = Fail
		check.Detail = fmt.Sprintf("No row on the %s sheet has the headers %s.", sheet, joinAnd(cols))
	case len(found[0].rows) == 0:
		check.Status = Fail
		check.Detail = fmt.Sprintf("The headers are at %s, but there are no rows under them.",
			found[0].where())
	default:
		check.Status = Pass
		check.Detail = fmt.Sprintf("%s, %d rows.", found[0].where(), len(found[0].rows))
	}
	j.add(check)
	if check.Status != Pass {
		return nil
	}
	return found[0]
}

// closestHeaders picks one column per header, taking the tightest span when a
// header repeats in the row, since a table's own columns sit together.
func closestHeaders(cols []string, at map[string][]int) map[string]int {
	var best map[string]int
	bestSpan := math.MaxInt
	pick := make(map[string]int, len(cols))
	var walk func(i, lo, hi int)
	walk = func(i, lo, hi int) {
		if i > 0 && hi-lo >= bestSpan {
			return
		}
		if i == len(cols) {
			best, bestSpan = maps.Clone(pick), hi-lo
			return
		}
		for _, c := range at[normHeader(cols[i])] {
			pick[cols[i]] = c
			if i == 0 {
				walk(1, c, c)
			} else {
				walk(i+1, min(lo, c), max(hi, c))
			}
		}
	}
	walk(0, 0, 0)
	return best
}

// dataRows reads down from the header to the first row where every expected
// column is blank. Rows hidden by a filter are left out of visible, since the
// filter is often the answer, but kept in all: a sort applies to the whole
// table, so the order check has to see them.
func (j *judging) dataRows(b *block) (visible, all []int) {
	for r := b.header + 1; r <= len(b.grid); r++ {
		blank := true
		for name := range b.col {
			if strings.TrimSpace(b.value(r, name)) != "" {
				blank = false
				break
			}
		}
		if blank {
			break
		}
		all = append(all, r)
		if shown, err := j.f.GetRowVisible(b.sheet, r); err == nil && !shown {
			continue
		}
		visible = append(visible, r)
	}
	return visible, all
}

// --- values ---------------------------------------------------------------

func (j *judging) checkValues(b *block) {
	ex := j.ex
	cols, want := ex.Result.Columns, ex.Result.Rows
	check := Check{ID: "values", Label: "Values match the expected result"}

	switch {
	case len(want) == 1 && !ex.Sample:
		if len(b.rows) != 1 {
			check.Items = append(check.Items, fmt.Sprintf("Expected one row, found %d.", len(b.rows)))
		}
		for _, c := range cols {
			if got := b.value(b.rows[0], c); !equal(got, want[0][c]) {
				check.Items = append(check.Items,
					fmt.Sprintf("%s is %s, expected %s.", c, shown(got), display(want[0][c])))
			}
		}
	case len(cols) > 1 && uniqueKeys(want, cols[0]):
		check.Items = keyedDiff(b, want, cols, ex.Sample)
	default:
		check.Items = multisetDiff(b, want, cols, ex.Sample)
	}

	if len(check.Items) > 0 {
		check.Status = Fail
	} else {
		check.Status = Pass
		check.Detail = fmt.Sprintf("All %d rows match.", len(want))
		if ex.Sample {
			check.Detail = fmt.Sprintf("All %d expected rows are there.", len(want))
		}
	}
	j.add(check)
}

// keyedDiff matches rows on the first column, so a mistake reads as "Ast CD:
// Total Workload is 3.5" rather than "a row is missing".
func keyedDiff(b *block, want []map[string]any, cols []string, sample bool) []string {
	key := cols[0]
	byKey := map[string][]int{}
	for _, r := range b.rows {
		k := keyText(b.value(r, key))
		byKey[k] = append(byKey[k], r)
	}

	var items []string
	expected := map[string]bool{}
	for _, w := range want {
		name := display(w[key])
		k := keyText(name)
		expected[k] = true
		rows := byKey[k]
		switch {
		case len(rows) == 0:
			items = append(items, fmt.Sprintf("No row for %s %s.", key, name))
		case len(rows) > 1:
			items = append(items, fmt.Sprintf("%s %s appears %d times.", key, name, len(rows)))
		default:
			for _, c := range cols[1:] {
				if got := b.value(rows[0], c); !equal(got, w[c]) {
					items = append(items, fmt.Sprintf("%s %s: %s is %s, expected %s.",
						key, name, c, shown(got), display(w[c])))
				}
			}
		}
	}
	if !sample {
		for _, r := range b.rows {
			if v := b.value(r, key); !expected[keyText(v)] {
				items = append(items, fmt.Sprintf("Extra row: %s %s.", key, shown(v)))
			}
		}
	}
	return items
}

// multisetDiff is for results whose first column repeats, so rows can only
// be matched whole.
func multisetDiff(b *block, want []map[string]any, cols []string, sample bool) []string {
	used := make([]bool, len(b.rows))
	var items []string
	for _, w := range want {
		matched := false
		for i, r := range b.rows {
			if used[i] || !rowEqual(b, r, w, cols) {
				continue
			}
			used[i], matched = true, true
			break
		}
		if !matched {
			items = append(items, "No row matching "+describeRow(w, cols)+".")
		}
	}
	if !sample {
		extra := 0
		for _, u := range used {
			if !u {
				extra++
			}
		}
		if extra > 0 {
			items = append(items, fmt.Sprintf("%d extra rows that are not in the expected result.", extra))
		}
	}
	return items
}

func rowEqual(b *block, r int, w map[string]any, cols []string) bool {
	for _, c := range cols {
		if !equal(b.value(r, c), w[c]) {
			return false
		}
	}
	return true
}

func equal(got string, want any) bool {
	got = strings.TrimSpace(got)
	switch w := want.(type) {
	case nil:
		return got == ""
	case bool:
		switch strings.ToUpper(got) {
		case "TRUE", "1":
			return w
		case "FALSE", "0":
			return !w
		}
		return false
	case float64:
		g, err := strconv.ParseFloat(got, 64)
		return err == nil && closeEnough(g, w)
	case string:
		ws := strings.TrimSpace(w)
		if wf, err := strconv.ParseFloat(ws, 64); err == nil {
			if g, err := strconv.ParseFloat(got, 64); err == nil {
				return closeEnough(g, wf)
			}
		}
		return got == ws
	}
	return got == fmt.Sprint(want)
}

// closeEnough allows the rounding a stated precision implies: an expected
// 1.78 accepts 1.7777…, while an expected 4 or 4.5 must be exact.
func closeEnough(got, want float64) bool {
	decimals := 0
	if _, frac, ok := strings.Cut(strconv.FormatFloat(want, 'f', -1, 64), "."); ok {
		decimals = len(frac)
	}
	tol := 1e-9 * math.Max(1, math.Abs(want))
	if decimals >= 2 {
		tol = 0.5*math.Pow(10, -float64(decimals)) + 1e-12
	}
	return math.Abs(got-want) <= tol
}

func uniqueKeys(rows []map[string]any, key string) bool {
	seen := map[string]bool{}
	for _, r := range rows {
		k := keyText(display(r[key]))
		if k == "(blank)" || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}

func keyText(s string) string {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return s
}

func display(v any) string {
	switch t := v.(type) {
	case nil:
		return "(blank)"
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if t == "" {
			return "(blank)"
		}
		return t
	}
	return fmt.Sprint(v)
}

func shown(got string) string {
	got = strings.TrimSpace(got)
	if got == "" {
		return "(blank)"
	}
	if f, err := strconv.ParseFloat(got, 64); err == nil {
		return strconv.FormatFloat(math.Round(f*1e6)/1e6, 'f', -1, 64)
	}
	return got
}

func describeRow(w map[string]any, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = c + " " + display(w[c])
	}
	return strings.Join(parts, ", ")
}

// --- order ----------------------------------------------------------------

func (j *judging) checkOrder(b *block) {
	ex := j.ex
	switch {
	case len(ex.SortedBy) > 0:
		check := Check{ID: "order", Label: "Sorted by " + describeSort(ex), Status: Pass}
		for i := 1; i < len(b.all); i++ {
			if compareCells(b, b.all[i-1], b.all[i], ex.SortedBy) > 0 {
				check.Status = Fail
				check.Detail = fmt.Sprintf("Rows %d and %d are out of order.", b.all[i-1], b.all[i])
				if len(b.all) > len(b.rows) {
					check.Detail += " Rows hidden by the filter count too: sort the whole table, then filter."
				}
				break
			}
		}
		j.add(check)

	case ex.RowOrder == "listed":
		key := ex.Result.Columns[0]
		var want []string
		inWant := map[string]bool{}
		for _, w := range ex.Result.Rows {
			k := keyText(display(w[key]))
			want = append(want, k)
			inWant[k] = true
		}
		var got []string
		for _, r := range b.rows {
			if k := keyText(b.value(r, key)); inWant[k] {
				got = append(got, k)
			}
		}
		check := Check{ID: "order", Label: "Rows in the order " + strings.Join(want, ", "), Status: Pass}
		if !slices.Equal(got, want) {
			check.Status = Fail
			check.Detail = "Found " + strings.Join(got, ", ") + "."
		}
		j.add(check)
	}
}

func compareCells(b *block, r1, r2 int, keys []string) int {
	for _, key := range keys {
		name := strings.TrimPrefix(key, "-")
		x, y := strings.TrimSpace(b.value(r1, name)), strings.TrimSpace(b.value(r2, name))
		var order int
		xf, xerr := strconv.ParseFloat(x, 64)
		yf, yerr := strconv.ParseFloat(y, 64)
		if xerr == nil && yerr == nil {
			order = cmp.Compare(xf, yf)
		} else {
			order = strings.Compare(strings.ToLower(x), strings.ToLower(y))
		}
		if strings.HasPrefix(key, "-") {
			order = -order
		}
		if order != 0 {
			return order
		}
	}
	return 0
}

func describeSort(ex datasets.Example) string {
	var parts []string
	for _, key := range ex.SortedBy {
		desc := strings.HasPrefix(key, "-")
		name := strings.TrimPrefix(key, "-")
		numeric := false
		if len(ex.Result.Rows) > 0 {
			_, numeric = ex.Result.Rows[0][name].(float64)
		}
		dir := "A to Z"
		switch {
		case numeric && desc:
			dir = "largest first"
		case numeric:
			dir = "smallest first"
		case desc:
			dir = "Z to A"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, dir))
	}
	return strings.Join(parts, ", then ")
}

// --- formulas -------------------------------------------------------------

func (j *judging) checkComputed(b *block) {
	if len(j.ex.Computed) == 0 {
		return
	}
	sf := j.formulas[b.sheet]
	check := Check{ID: "computed", Label: "Answers come from formulas, not typing", Status: Pass}
	for _, name := range j.ex.Computed {
		typed := 0
		for _, r := range b.rows {
			if !sf.computed(b.col[name], r) {
				typed++
			}
		}
		if typed > 0 {
			check.Items = append(check.Items,
				fmt.Sprintf("%s: %d of %d cells are typed values.", name, typed, len(b.rows)))
		}
	}
	if len(check.Items) > 0 {
		check.Status = Fail
		check.Detail = "A typed value is right today and wrong as soon as the data changes."
	} else {
		check.Detail = "Every cell in " + joinAnd(j.ex.Computed) + " is calculated."
	}
	j.add(check)
}

var (
	functionShaped = regexp.MustCompile(`^[A-Z][A-Z.]+$`)
	structuredRef  = regexp.MustCompile(`[A-Za-z_\\][\w.]*\[|\[@`)
	xlPrefixes     = strings.NewReplacer("_XLFN._XLWS.", "", "_XLFN.", "", "_XLWS.", "")
)

func (j *judging) checkTechniques() {
	texts := j.studentFormulas()
	var checked, missing []string
	for _, req := range j.ex.Requires {
		switch {
		case functionShaped.MatchString(req):
			checked = append(checked, req)
			if !usesFunction(texts, req) {
				missing = append(missing, req)
			}
		case req == "structured references":
			checked = append(checked, req)
			if !slices.ContainsFunc(texts, structuredRef.MatchString) {
				missing = append(missing, req)
			}
		default:
			j.notChecked = append(j.notChecked,
				"Whether you used "+req+". The checker cannot see that in a formula.")
		}
	}
	if len(checked) == 0 {
		return
	}
	check := Check{ID: "techniques", Label: "Uses " + joinAnd(checked), Status: Pass}
	if len(missing) > 0 {
		check.Status = Fail
		check.Detail = "No formula on your sheets uses " + joinAnd(missing) + "."
	}
	j.add(check)
}

// studentFormulas is every formula outside the sheets the app wrote.
func (j *judging) studentFormulas() []string {
	var texts []string
	for name, sf := range j.formulas {
		if name == workbook.SheetBrief || name == workbook.SheetExpected {
			continue
		}
		texts = append(texts, sf.texts...)
	}
	return texts
}

// usesFunction matches a whole function name, so SUMIFS does not count as
// SUMIF. Excel saves newer functions with prefixes like _xlfn., stripped here.
func usesFunction(texts []string, name string) bool {
	re := regexp.MustCompile(`(^|[^A-Z0-9_.])` + regexp.QuoteMeta(name) + `\s*\(`)
	for _, t := range texts {
		if re.MatchString(xlPrefixes.Replace(strings.ToUpper(t))) {
			return true
		}
	}
	return false
}

// --- Data-sheet checks ----------------------------------------------------

func (j *judging) checkTable() {
	check := Check{ID: "table", Label: "The data is an Excel Table", Status: Pass}
	tables, err := j.f.GetTables(workbook.SheetData)
	if err != nil || len(tables) == 0 {
		check.Status = Fail
		check.Detail = "No Table on the Data sheet. Click inside the data and press Ctrl+T."
	} else {
		check.Detail = fmt.Sprintf("%s covers %s.", tables[0].Name, tables[0].Range)
	}
	j.add(check)
}

var formulaTags = regexp.MustCompile(`</?formula[12]>`)

func (j *judging) checkValidation() {
	check := Check{ID: "validation", Label: "Validation rules are on the Data sheet", Status: Pass}
	dvs, err := j.f.GetDataValidations(workbook.SheetData)
	grid, gerr := j.f.GetRows(workbook.SheetData, excelize.Options{RawCellValue: true})
	if err != nil || gerr != nil || len(grid) == 0 {
		check.Status, check.Detail = Fail, "Could not read the Data sheet."
		j.add(check)
		return
	}
	colOf := map[string]int{}
	for c, v := range grid[0] {
		colOf[normHeader(v)] = c + 1
	}

	for _, want := range j.ex.Result.Rows {
		field, rule := display(want["Field"]), display(want["Rule"])
		col, ok := colOf[normHeader(field)]
		if !ok {
			check.Items = append(check.Items, field+" is not a column on the Data sheet.")
			continue
		}
		dv := validationFor(dvs, col)
		switch {
		case dv == nil:
			check.Items = append(check.Items, field+": no validation rule.")
		case strings.HasPrefix(rule, "List") && dv.Type != "list":
			check.Items = append(check.Items, fmt.Sprintf("%s: the rule is %q, expected a list.", field, dv.Type))
		case strings.HasPrefix(rule, "Decimal"):
			lo, hi, _ := strings.Cut(display(want["Allowed"]), " to ")
			gotLo, gotHi := formulaValue(dv.Formula1), formulaValue(dv.Formula2)
			switch {
			case dv.Type != "decimal":
				check.Items = append(check.Items, fmt.Sprintf("%s: the rule is %q, expected decimal.", field, dv.Type))
			case dv.Operator != "" && dv.Operator != "between":
				check.Items = append(check.Items, fmt.Sprintf("%s: the rule uses %q, expected between.", field, dv.Operator))
			case gotLo != keyText(lo) || gotHi != keyText(hi):
				check.Items = append(check.Items, fmt.Sprintf("%s: allows %s to %s, expected %s to %s.",
					field, gotLo, gotHi, lo, hi))
			}
		}
	}
	if len(check.Items) > 0 {
		check.Status = Fail
	}
	j.add(check)
	j.notChecked = append(j.notChecked, "Which values each list allows. The checker only confirms it is a list.")
}

func validationFor(dvs []*excelize.DataValidation, col int) *excelize.DataValidation {
	for _, dv := range dvs {
		for _, ref := range strings.Fields(dv.Sqref) {
			if a, ok := parseArea(ref); ok && a.contains(col, 2) {
				return dv
			}
		}
	}
	return nil
}

func formulaValue(s string) string {
	return keyText(strings.Trim(formulaTags.ReplaceAllString(s, ""), `" `))
}

// --- text helpers ---------------------------------------------------------

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func missingFrom(all, have []string) []string {
	var out []string
	for _, a := range all {
		if !slices.Contains(have, a) {
			out = append(out, a)
		}
	}
	return out
}
