package datasets

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Recomputing every worked answer from the data it claims to come from.
//
// Until this existed, an "expected result" was a claim somebody typed. Nothing
// checked it, and W1D5 shipped with an answer its own formula could not
// produce. Worse than a one-off wrong answer is the slow kind: edit one row of
// datasets.json and every stored result silently goes stale, while the page
// keeps showing them and the tutor keeps being handed them as ground truth.
//
// Each encoder below expresses the same logic as the case's Excel formula, in
// Go. The test then diffs. That makes the answers derivations rather than
// assertions.
//
// The honest limit: this is a re-implementation, so a misunderstanding shared
// between the formula and the encoder will agree with itself. It catches drift,
// typos and shape errors — not a wrong idea. Only reading the case catches that.
//
// W2D2 has no encoder: its "result" is a table of data-validation rules, which
// is configuration to set up rather than an answer computed from the data.

// norm renders a cell for comparison, flattening the int/float distinction that
// survives a JSON round trip — a stored 2 and a computed 2.0 are the same
// answer, and a test that says otherwise is noise.
func norm(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func str(v any) string { return norm(v) }

// round2 matches what a sheet shows at two decimal places.
func round2(f float64) float64 { return math.Round(f*100) / 100 }

type computed struct {
	columns []string
	rows    []map[string]any
}

// --- one encoder per case, mirroring its formula --------------------------

// W1D1: the first 20 rows of five columns, retyped. The answer is the data.
func w1d1(t Table) computed {
	cols := []string{"No", "Course ID", "Class", "Ast", "Workload"}
	rows := make([]map[string]any, 0, 20)
	for _, r := range t.Rows[:20] {
		row := map[string]any{}
		for _, c := range cols {
			row[c] = r[c]
		}
		rows = append(rows, row)
	}
	return computed{cols, rows}
}

// W1D2: =$A2*B$1 — each assistant's total workload against each multiplier.
func w1d2(t Table) computed {
	multipliers := []struct {
		label string
		value float64
	}{{"1.0", 1.0}, {"1.25", 1.25}, {"1.5", 1.5}, {"2.0", 2.0}}

	total := map[string]float64{}
	for _, r := range t.Rows {
		total[str(r["Ast"])] += num(r["Workload"])
	}
	names := keysSorted(total)

	cols := []string{"Ast"}
	for _, m := range multipliers {
		cols = append(cols, m.label)
	}
	rows := make([]map[string]any, 0, 6)
	for _, name := range names[:6] {
		row := map[string]any{"Ast": name}
		for _, m := range multipliers {
			row[m.label] = total[name] * m.value
		}
		rows = append(rows, row)
	}
	return computed{cols, rows}
}

// W1D3: =IFS(Workload<1.5,"light",Workload<2.5,"standard",TRUE,"heavy").
// The cutoffs are the control cells' default values.
func w1d3(t Table) computed {
	const light, heavy = 1.5, 2.5
	cols := []string{"No", "Ast", "Workload", "Band"}
	rows := make([]map[string]any, 0, 12)
	for _, r := range t.Rows[:12] {
		w := num(r["Workload"])
		band := "heavy"
		switch {
		case w < light:
			band = "light"
		case w < heavy:
			band = "standard"
		}
		rows = append(rows, map[string]any{
			"No": r["No"], "Ast": r["Ast"], "Workload": r["Workload"], "Band": band,
		})
	}
	return computed{cols, rows}
}

// W1D4: COUNTIFS / SUMIFS per Group, plus a count of one lecturer type.
func w1d4(t Table) computed {
	type agg struct {
		sessions, vc int
		workload     float64
	}
	by := map[string]*agg{}
	for _, r := range t.Rows {
		g := str(r["Group"])
		if by[g] == nil {
			by[g] = &agg{}
		}
		by[g].sessions++
		by[g].workload += num(r["Workload"])
		if str(r["Lecturer Type"]) == "VC" {
			by[g].vc++
		}
	}

	cols := []string{"Group", "Sessions", "Total Workload", "VC Sessions", "Avg Workload"}
	rows := make([]map[string]any, 0, len(by))
	for _, g := range keysSortedPtr(by) {
		a := by[g]
		rows = append(rows, map[string]any{
			"Group": g, "Sessions": float64(a.sessions),
			"Total Workload": a.workload, "VC Sessions": float64(a.vc),
			"Avg Workload": round2(a.workload / float64(a.sessions)),
		})
	}
	return computed{cols, rows}
}

// W1D5: =SORT(UNIQUE(tblTeaching[Ast])) — the first ten of the spill.
func w1d5(t Table) computed {
	seen := map[string]bool{}
	for _, r := range t.Rows {
		seen[str(r["Ast"])] = true
	}
	names := keysSortedBool(seen)
	rows := make([]map[string]any, 0, 10)
	for _, n := range names[:10] {
		rows = append(rows, map[string]any{"Ast": n})
	}
	return computed{[]string{"Ast"}, rows}
}

// W2D1: SUMIFS over the Table, per lecturer type, heaviest first.
func w2d1(t Table) computed {
	total := map[string]float64{}
	for _, r := range t.Rows {
		total[str(r["Lecturer Type"])] += num(r["Workload"])
	}
	types := keysSorted(total)
	sort.SliceStable(types, func(i, j int) bool { return total[types[i]] > total[types[j]] })

	rows := make([]map[string]any, 0, len(types))
	for _, ty := range types {
		rows = append(rows, map[string]any{"Lecturer Type": ty, "Total Workload": total[ty]})
	}
	return computed{[]string{"Lecturer Type", "Total Workload"}, rows}
}

// W2D3: sort by Group then Workload descending, filter to VC on Database Systems.
func w2d3(t Table) computed {
	cols := []string{"Group", "Course Name", "Ast", "Lecturer Type", "Workload"}
	var kept []map[string]any
	for _, r := range t.Rows {
		if str(r["Lecturer Type"]) == "VC" && str(r["Course Name"]) == "Database Systems" {
			row := map[string]any{}
			for _, c := range cols {
				row[c] = r[c]
			}
			kept = append(kept, row)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		gi, gj := str(kept[i]["Group"]), str(kept[j]["Group"])
		if gi != gj {
			return gi < gj
		}
		return num(kept[i]["Workload"]) > num(kept[j]["Workload"])
	})
	return computed{cols, kept}
}

// W2D4: LEFT / MID / RIGHT over the distinct Course IDs, dept code 4 long.
func w2d4(t Table) computed {
	const dept = 4
	cols := []string{"Course ID", "Dept", "Number", "Section"}
	seen := map[string]bool{}
	var rows []map[string]any
	for _, r := range t.Rows {
		id := str(r["Course ID"])
		if seen[id] || len(id) < dept+4 {
			continue
		}
		seen[id] = true
		rows = append(rows, map[string]any{
			"Course ID": id,
			"Dept":      id[:dept],
			"Number":    id[dept : dept+4],
			"Section":   id[dept+4:],
		})
		if len(rows) == 8 {
			break
		}
	}
	return computed{cols, rows}
}

// W2D5: =IFERROR(IF(SEARCH(term,name)>0,TRUE,""),"") over the first ten names.
// SEARCH is case-insensitive and 1-based; a miss is blank, never #VALUE!.
func w2d5(t Table) computed {
	const term = "Database"
	cols := []string{"Course Name", "Position", "Flag"}
	rows := make([]map[string]any, 0, 10)
	for _, r := range t.Rows[:10] {
		name := str(r["Course Name"])
		at := strings.Index(strings.ToLower(name), strings.ToLower(term))
		position, flag := any(""), any("")
		if at >= 0 {
			position = float64(at + 1)
			flag = true
		}
		rows = append(rows, map[string]any{
			"Course Name": name, "Position": position, "Flag": flag,
		})
	}
	return computed{cols, rows}
}

func keysSorted(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysSortedBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysSortedPtr[T any](m map[string]*T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestStoredResultsMatchTheData recomputes each answer and diffs it against
// what examples.json claims.
func TestStoredResultsMatchTheData(t *testing.T) {
	encoders := map[string]func(Table) computed{
		"W1D1": w1d1, "W1D2": w1d2, "W1D3": w1d3, "W1D4": w1d4, "W1D5": w1d5,
		"W2D1": w2d1, "W2D3": w2d3, "W2D4": w2d4, "W2D5": w2d5,
		"W3D1": w3d1, "W3D2": w3d2, "W3D3": w3d3, "W3D4": w3d4, "W3D5": w3d5,
		"W4D1": w4d1, "W4D2": w4d2, "W4D3": w4d3, "W4D4": w4d4, "W4D5": w4d5,
		"W5D1": w5d1, "W5D2": w5d2, "W5D3": w5d3, "W5D4": w5d4, "W5D5": w5d5,
		"W6D1": w6d1, "W6D2": w6d2, "W6D3": w6d3, "W6D4": w6d4, "W6D5": w6d5,
		"W1D6": w1d6, "W2D6": w2d6, "W3D6": w3d6, "W4D6": w4d6,
		"W7D1": w7d1, "W7D2": w7d2, "W7D3": w7d3, "W7D4": w7d4, "W7D5": w7d5,
		"W8D1": w8d1, "W8D2": w8d2, "W8D3": w8d3, "W8D4": w8d4, "W8D5": w8d5, "W8D6": w8d6,
		"W9D1": w9d1, "W9D2": w9d2, "W9D3": w9d3, "W9D4": w9d4, "W9D5": w9d5,
	}

	all := AllExamples()
	for _, slug := range sorted(all) {
		encode, ok := encoders[slug]
		if !ok {
			continue
		}
		ex := all[slug]
		table, ok := Get(ex.DatasetID)
		if !ok {
			t.Errorf("%s: dataset %q missing", slug, ex.DatasetID)
			continue
		}

		got := encode(table)
		want := ex.Result

		if strings.Join(got.columns, "|") != strings.Join(want.Columns, "|") {
			t.Errorf("%s: columns\n  computed %v\n  stored   %v",
				slug, got.columns, want.Columns)
			continue
		}
		if len(got.rows) != len(want.Rows) {
			t.Errorf("%s: computed %d rows, stored %d",
				slug, len(got.rows), len(want.Rows))
			continue
		}
		for i := range got.rows {
			for _, c := range got.columns {
				g, w := norm(got.rows[i][c]), norm(want.Rows[i][c])
				if g != w {
					t.Errorf("%s row %d %q: computed %q, stored %q", slug, i, c, g, w)
				}
			}
		}
	}

	if len(encoders)+1 != len(all) {
		t.Logf("NOTE: %d of %d examples have an encoder; the rest state "+
			"configuration rather than a computed answer", len(encoders), len(all))
	}
}

// --- weeks 3 and 4 --------------------------------------------------------

// properCase mirrors Excel's PROPER: first letter of each word up, rest down.
func properCase(s string) string {
	words := strings.Fields(s) // Fields also collapses the runs TRIM removes
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

const w3d1Sep = " - "

// W3D1: a label joined from three columns by a separator held in a cell.
func w3d1(t Table) computed {
	cols := []string{"Ast", "Course Name", "Class"}
	rows := make([]map[string]any, 0, 8)
	for _, r := range t.Rows[:8] {
		row := map[string]any{}
		parts := make([]string, 0, len(cols))
		for _, c := range cols {
			row[c] = r[c]
			parts = append(parts, str(r[c]))
		}
		row["Label"] = strings.Join(parts, w3d1Sep)
		rows = append(rows, row)
	}
	return computed{append(cols, "Label"), rows}
}

// W3D2: PROPER over TRIM, and whether it changed anything. On a clean roster
// nothing changes — which is the point: the check passed.
func w3d2(t Table) computed {
	rows := make([]map[string]any, 0, 10)
	for _, r := range t.Rows[:10] {
		name := str(r["Name"])
		changed := ""
		if properCase(name) != name {
			changed = "changed"
		}
		rows = append(rows, map[string]any{
			"Inisial": r["Inisial"], "Name": name,
			"Cleaned": properCase(name), "Changed": changed,
		})
	}
	return computed{[]string{"Inisial", "Name", "Cleaned", "Changed"}, rows}
}

// W3D3: Time split on its delimiter.
func w3d3(t Table) computed {
	rows := make([]map[string]any, 0, 8)
	for _, r := range t.Rows[:8] {
		parts := strings.SplitN(str(r["Time"]), " - ", 2)
		row := map[string]any{"No": r["No"], "Time": r["Time"], "Start": parts[0], "End": ""}
		if len(parts) == 2 {
			row["End"] = parts[1]
		}
		rows = append(rows, row)
	}
	return computed{[]string{"No", "Time", "Start", "End"}, rows}
}

// W3D4: INDEX MATCH by initials, including a code that is not on the roster.
func w3d4(t Table) computed {
	const missing = "not on the roster"
	by := map[string]map[string]any{}
	for _, r := range t.Rows {
		by[str(r["Inisial"])] = r
	}
	rows := make([]map[string]any, 0, 4)
	for _, code := range []string{"HI", "AB", "TU", "ZZ"} {
		hit, ok := by[code]
		row := map[string]any{"Code": code, "Name": missing, "Wed": missing}
		if ok {
			row["Name"], row["Wed"] = hit["Name"], hit["Wed"]
		}
		rows = append(rows, row)
	}
	return computed{[]string{"Code", "Name", "Wed"}, rows}
}

var w3d5Days = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// W3D5: the assistant-by-day workload matrix the two-way lookup reads from.
func w3d5(t Table) computed {
	grid := map[string]float64{}
	seen := map[string]bool{}
	for _, r := range t.Rows {
		grid[str(r["Ast"])+"|"+str(r["Day"])] += num(r["Workload"])
		seen[str(r["Ast"])] = true
	}
	names := keysSortedBool(seen)

	rows := make([]map[string]any, 0, 8)
	for _, a := range names[:8] {
		row := map[string]any{"Ast": a}
		for _, d := range w3d5Days {
			row[d] = grid[a+"|"+d] // absent pairs are a real zero, not a gap
		}
		rows = append(rows, row)
	}
	return computed{append([]string{"Ast"}, w3d5Days...), rows}
}

// W4D1: XLOOKUP in next-smaller mode against a band table.
func w4d1(t Table) computed {
	floors := []struct {
		at   float64
		name string
	}{{0, "light"}, {1.5, "standard"}, {2.5, "heavy"}}

	rows := make([]map[string]any, 0, 10)
	for _, r := range t.Rows[:10] {
		w := num(r["Workload"])
		band := "unbanded"
		for _, f := range floors {
			if w >= f.at {
				band = f.name
			}
		}
		rows = append(rows, map[string]any{
			"No": r["No"], "Ast": r["Ast"], "Workload": r["Workload"], "Band": band,
		})
	}
	return computed{[]string{"No", "Ast", "Workload", "Band"}, rows}
}

// W4D2: wildcard XLOOKUP returns the first match in table order, not the best.
func w4d2(t Table) computed {
	const missing = "no course matches"
	rows := make([]map[string]any, 0, 4)
	for _, pat := range []string{"*Data*", "*Net*", "*Program*", "*Chemistry*"} {
		core := strings.ToLower(strings.Trim(pat, "*"))
		row := map[string]any{"Search": pat, "First match": missing, "Course ID": missing}
		for _, r := range t.Rows {
			if strings.Contains(strings.ToLower(str(r["Course Name"])), core) {
				row["First match"], row["Course ID"] = r["Course Name"], r["Course ID"]
				break
			}
		}
		rows = append(rows, row)
	}
	return computed{[]string{"Search", "First match", "Course ID"}, rows}
}

// W4D3: the same answers as INDEX MATCH, reached with the legacy pair.
func w4d3(t Table) computed {
	short := map[string]string{
		"Monday": "Mon", "Tuesday": "Tue", "Wednesday": "Wed",
		"Thursday": "Thu", "Friday": "Fri", "Saturday": "Sat",
	}
	by := map[string]map[string]any{}
	for _, r := range t.Rows {
		by[str(r["Inisial"])] = r
	}
	probes := [][2]string{{"AB", "Monday"}, {"HI", "Thursday"}, {"TU", "Saturday"}}

	rows := make([]map[string]any, 0, len(probes))
	for _, p := range probes {
		hit := by[p[0]]
		rows = append(rows, map[string]any{
			"Code": p[0], "Name": hit["Name"], "Day": p[1], "Shift": hit[short[p[1]]],
		})
	}
	return computed{[]string{"Code", "Name", "Day", "Shift"}, rows}
}

// W4D4: two keys, and a pair that matches nothing.
func w4d4(t Table) computed {
	grid := map[string]float64{}
	for _, r := range t.Rows {
		grid[str(r["Ast"])+"|"+str(r["Day"])] += num(r["Workload"])
	}
	probes := [][2]string{{"AB", "Monday"}, {"BC", "Tuesday"}, {"HI", "Friday"}, {"AB", "Sunday"}}

	rows := make([]map[string]any, 0, len(probes))
	for _, p := range probes {
		row := map[string]any{"Ast": p[0], "Day": p[1], "Workload": any("no session")}
		if total, ok := grid[p[0]+"|"+p[1]]; ok {
			row["Workload"] = total
		}
		rows = append(rows, row)
	}
	return computed{[]string{"Ast", "Day", "Workload"}, rows}
}

// W4D5: UNIQUE, SUMIFS, FILTER and SORT working as one spilled formula.
func w4d5(t Table) computed {
	const threshold = 4.0
	total := map[string]float64{}
	for _, r := range t.Rows {
		total[str(r["Ast"])] += num(r["Workload"])
	}

	over := make([]string, 0, len(total))
	for a, w := range total {
		if w >= threshold {
			over = append(over, a)
		}
	}
	sort.Strings(over) // ties break alphabetically, which is SORT's own behaviour
	sort.SliceStable(over, func(i, j int) bool { return total[over[i]] > total[over[j]] })

	rows := make([]map[string]any, 0, len(over))
	for _, a := range over {
		rows = append(rows, map[string]any{"Ast": a, "Total Workload": total[a]})
	}
	return computed{[]string{"Ast", "Total Workload"}, rows}
}

// --- weeks 5 and 6 --------------------------------------------------------

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

// uniqueInOrder mirrors UNIQUE: first appearance order, not sorted.
func uniqueInOrder(t Table, col string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range t.Rows {
		v := str(r[col])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func totalBy(t Table, key string) map[string]float64 {
	total := map[string]float64{}
	for _, r := range t.Rows {
		total[str(r[key])] += num(r["Workload"])
	}
	return total
}

// W5D1: =UNIQUE(Ast) with SUMIF beside it.
func w5d1(t Table) computed {
	total := totalBy(t, "Ast")
	var rows []map[string]any
	for _, a := range uniqueInOrder(t, "Ast") {
		rows = append(rows, map[string]any{"Ast": a, "Total Workload": total[a]})
	}
	return computed{[]string{"Ast", "Total Workload"}, rows}
}

// W5D2: SUMIFS matrix, Group down, Lecturer Type across.
func w5d2(t Table) computed {
	grid := map[string]float64{}
	for _, r := range t.Rows {
		grid[str(r["Group"])+"|"+str(r["Lecturer Type"])] += num(r["Workload"])
	}
	var rows []map[string]any
	for _, g := range uniqueInOrder(t, "Group") {
		rows = append(rows, map[string]any{
			"Group": g, "VC": grid[g+"|VC"], "Forum": grid[g+"|Forum"],
		})
	}
	return computed{[]string{"Group", "VC", "Forum"}, rows}
}

// W5D3: the W5D1 summary through SORT(..., 2, -1). SORT is stable, so ties
// keep UNIQUE's order.
func w5d3(t Table) computed {
	c := w5d1(t)
	sort.SliceStable(c.rows, func(i, j int) bool {
		return num(c.rows[i]["Total Workload"]) > num(c.rows[j]["Total Workload"])
	})
	return c
}

// W5D4: COUNTIF per Group and its share of all sessions, largest first.
func w5d4(t Table) computed {
	count := map[string]float64{}
	for _, r := range t.Rows {
		count[str(r["Group"])]++
	}
	var rows []map[string]any
	for _, g := range uniqueInOrder(t, "Group") {
		rows = append(rows, map[string]any{
			"Group": g, "Sessions": count[g],
			"Share": round4(count[g] / float64(len(t.Rows))),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return num(rows[i]["Sessions"]) > num(rows[j]["Sessions"])
	})
	return computed{[]string{"Group", "Sessions", "Share"}, rows}
}

// W5D5: per day, COUNTIF sessions and SUMIF workload, Monday to Saturday.
func w5d5(t Table) computed {
	count := map[string]float64{}
	for _, r := range t.Rows {
		count[str(r["Day"])]++
	}
	total := totalBy(t, "Day")
	var rows []map[string]any
	for _, d := range w3d5Days {
		rows = append(rows, map[string]any{"Day": d, "Sessions": count[d], "Workload": total[d]})
	}
	return computed{[]string{"Day", "Sessions", "Workload"}, rows}
}

// W6D1: a title built from a count, and INDEX/MATCH on the MAX. MATCH takes
// the first maximum, so a tie goes to whoever UNIQUE listed first.
func w6d1(t Table) computed {
	names := uniqueInOrder(t, "Ast")
	total := totalBy(t, "Ast")
	top := names[0]
	for _, a := range names {
		if total[a] > total[top] {
			top = a
		}
	}
	return computed{[]string{"Title", "Most loaded"}, []map[string]any{{
		"Title":       fmt.Sprintf("Workload by Assistant - %d TAs", len(names)),
		"Most loaded": top,
	}}}
}

var weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// W6D2: COUNTIF of each shift code per weekday, plus the column totals.
func w6d2(t Table) computed {
	totals := map[string]any{"Code": "Total"}
	var rows []map[string]any
	for _, code := range []string{"P", "N", "M"} {
		row := map[string]any{"Code": code}
		for _, d := range weekdays {
			n := 0.0
			for _, r := range t.Rows {
				if str(r[d]) == code {
					n++
				}
			}
			row[d] = n
			prev, _ := totals[d].(float64)
			totals[d] = prev + n
		}
		rows = append(rows, row)
	}
	rows = append(rows, totals)
	return computed{append([]string{"Code"}, weekdays...), rows}
}

// W6D3: shift codes mapped to 1/2/3 by XLOOKUP, blank days to 0.
func w6d3(t Table) computed {
	code := map[string]float64{"P": 1, "N": 2, "M": 3}
	var rows []map[string]any
	for _, r := range t.Rows {
		row := map[string]any{"Inisial": r["Inisial"]}
		for _, d := range weekdays {
			row[d] = code[str(r[d])]
		}
		rows = append(rows, row)
	}
	return computed{append([]string{"Inisial"}, weekdays...), rows}
}

// W6D4: the four KPI cards.
func w6d4(t Table) computed {
	total := 0.0
	for _, r := range t.Rows {
		total += num(r["Workload"])
	}
	people := float64(len(uniqueInOrder(t, "Ast")))
	return computed{
		[]string{"Total sessions", "Total workload", "Assistants", "Avg load"},
		[]map[string]any{{
			"Total sessions": float64(len(t.Rows)), "Total workload": total,
			"Assistants": people, "Avg load": round2(total / people),
		}},
	}
}

const w6d5Group = "G2"

// W6D5: SUMIFS per assistant, keyed to the Group in the control cell.
func w6d5(t Table) computed {
	total := map[string]float64{}
	for _, r := range t.Rows {
		if str(r["Group"]) == w6d5Group {
			total[str(r["Ast"])] += num(r["Workload"])
		}
	}
	var rows []map[string]any
	for _, a := range uniqueInOrder(t, "Ast") {
		rows = append(rows, map[string]any{"Ast": a, "Workload": total[a]})
	}
	return computed{[]string{"Ast", "Workload"}, rows}
}
