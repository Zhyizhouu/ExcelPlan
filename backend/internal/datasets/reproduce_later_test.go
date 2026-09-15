package datasets

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Weekly builds and weeks 7 to 9. Most of these are references rather than
// judged cases, since the answer is a pivot, but the numbers still come from
// the data rather than from memory.

const grandTotal = "Grand Total"

func sum(m map[string]float64) float64 {
	total := 0.0
	for _, v := range m {
		total += v
	}
	return total
}

func sessionsBy(t Table, key string) map[string]float64 {
	count := map[string]float64{}
	for _, r := range t.Rows {
		count[str(r[key])]++
	}
	return count
}

// sortedTotals orders labels the way a pivot sorts Largest to Smallest: by
// value, with ties keeping the pivot's A to Z label order.
func sortedTotals(total map[string]float64) []string {
	names := keysSorted(total)
	sort.SliceStable(names, func(i, j int) bool { return total[names[i]] > total[names[j]] })
	return names
}

// oneWay is a single-field pivot with a Grand Total row.
func oneWay(labels []string, total map[string]float64, valueCol string) computed {
	rows := make([]map[string]any, 0, len(labels)+1)
	for _, l := range labels {
		rows = append(rows, map[string]any{"Row Labels": l, valueCol: total[l]})
	}
	rows = append(rows, map[string]any{"Row Labels": grandTotal, valueCol: sum(total)})
	return computed{[]string{"Row Labels", valueCol}, rows}
}

// pivotGrid is a two-way pivot with a Grand Total column and row. Empty
// combinations show 0, the setting W7D4 teaches.
func pivotGrid(rowKeys, colKeys []string, cell func(row, col string) float64) computed {
	cols := append(append([]string{"Row Labels"}, colKeys...), grandTotal)
	colTotal := map[string]float64{}
	all := 0.0
	var rows []map[string]any
	for _, rk := range rowKeys {
		row := map[string]any{"Row Labels": rk}
		rowTotal := 0.0
		for _, ck := range colKeys {
			v := cell(rk, ck)
			row[ck] = v
			rowTotal += v
			colTotal[ck] += v
		}
		row[grandTotal] = rowTotal
		all += rowTotal
		rows = append(rows, row)
	}
	last := map[string]any{"Row Labels": grandTotal, grandTotal: all}
	for _, ck := range colKeys {
		last[ck] = colTotal[ck]
	}
	return computed{cols, append(rows, last)}
}

func rosterByInisial() map[string]map[string]any {
	ws, _ := Get("workshift")
	out := make(map[string]map[string]any, len(ws.Rows))
	for _, r := range ws.Rows {
		out[str(r["Inisial"])] = r
	}
	return out
}

func firstRow(t Table, match func(map[string]any) bool) map[string]any {
	for _, r := range t.Rows {
		if match(r) {
			return r
		}
	}
	return nil
}

func one(v float64) string { return fmt.Sprintf("%.1f", v) }

func joinAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// --- weekly builds --------------------------------------------------------

const w2d6Keyword = "Data"

// W2D6: the self-cleaning table's derived columns, first eight rows.
func w2d6(t Table) computed {
	cols := []string{"Course ID", "Course Name", "Dept", "Number", "Section", "Flag"}
	var rows []map[string]any
	for _, r := range t.Rows[:8] {
		id, name := str(r["Course ID"]), str(r["Course Name"])
		flag := any("")
		if strings.Contains(strings.ToLower(name), strings.ToLower(w2d6Keyword)) {
			flag = true
		}
		rows = append(rows, map[string]any{
			"Course ID": id, "Course Name": name,
			"Dept": id[:4], "Number": id[4:8], "Section": id[8:], "Flag": flag,
		})
	}
	return computed{cols, rows}
}

// excelTrim mirrors TRIM: only character 32 counts as a space.
func excelTrim(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == ' ' }), " ")
}

func matchName(t Table, name string) string {
	for _, r := range t.Rows {
		if strings.EqualFold(str(r["Name"]), name) { // MATCH ignores case
			return str(r["Inisial"])
		}
	}
	return "not found"
}

// W3D6: each imported name looked up before and after cleaning.
func w3d6(t Table) computed {
	var rows []map[string]any
	for _, r := range t.Rows {
		raw := str(r["Imported Name"])
		if raw == "" {
			continue
		}
		cleaned := properCase(excelTrim(strings.ReplaceAll(raw, "\u00a0", " ")))
		rows = append(rows, map[string]any{
			"Imported Name": raw, "Cleaned": cleaned,
			"Before": matchName(t, raw), "After": matchName(t, cleaned),
		})
	}
	return computed{[]string{"Imported Name", "Cleaned", "Before", "After"}, rows}
}

// W4D6: four questions, each answered by the lookup that fits it.
func w4d6(t Table) computed {
	no7 := firstRow(t, func(r map[string]any) bool { return num(r["No"]) == 7 })
	net := firstRow(t, func(r map[string]any) bool {
		return strings.Contains(strings.ToLower(str(r["Course Name"])), "net")
	})
	hi := firstRow(t, func(r map[string]any) bool { return str(r["Ast"]) == "HI" })
	hiFri := firstRow(t, func(r map[string]any) bool {
		return str(r["Ast"]) == "HI" && str(r["Day"]) == "Friday"
	})
	friday := any("no session")
	if hiFri != nil {
		friday = hiFri["Workload"]
	}

	cols := []string{"Question", "Right tool", "Answer", "Why the others lose"}
	return computed{cols, []map[string]any{
		{"Question": "Course name for session No 7", "Right tool": "XLOOKUP, exact match",
			"Answer":              no7["Course Name"],
			"Why the others lose": "VLOOKUP counts columns, so inserting one returns a wrong value with no error."},
		{"Question": "First course whose name contains \"Net\"", "Right tool": "XLOOKUP, match mode 2",
			"Answer":              net["Course Name"],
			"Why the others lose": "An exact match on \"Net\" returns #N/A; a wildcard only works in match mode 2."},
		{"Question": "Course ID for assistant HI", "Right tool": "INDEX and MATCH",
			"Answer":              hi["Course ID"],
			"Why the others lose": "Course ID sits left of Ast, and VLOOKUP cannot read left of its key."},
		{"Question": "Workload for HI on Friday", "Right tool": "XLOOKUP on two conditions",
			"Answer":              friday,
			"Why the others lose": "A one-key lookup cannot hold two criteria, and VLOOKUP has no way to combine them."},
	}}
}

// W1D6: the assistant card for three codes, the last not on the roster.
func w1d6(t Table) computed {
	const light, standard = 3.0, 4.5
	// The multipliers are the headers themselves, so one mixed reference reads them.
	cols := []string{"Code", "Sessions", "Total Workload", "First course", "Band", "1.25", "1.5", "2"}
	var rows []map[string]any
	for _, code := range []string{"AB", "CD", "ZZ"} {
		sessions, total := 0.0, 0.0
		first, band := any("not on roster"), "not on roster"
		for _, r := range t.Rows {
			if str(r["Ast"]) != code {
				continue
			}
			if sessions == 0 {
				first = r["Course Name"]
			}
			sessions++
			total += num(r["Workload"])
		}
		if sessions > 0 {
			switch {
			case total < light:
				band = "light"
			case total < standard:
				band = "standard"
			default:
				band = "heavy"
			}
		}
		rows = append(rows, map[string]any{
			"Code": code, "Sessions": sessions, "Total Workload": total, "First course": first,
			"Band": band, "1.25": total * 1.25, "1.5": total * 1.5, "2": total * 2,
		})
	}
	return computed{cols, rows}
}

// --- week 7 ---------------------------------------------------------------

// W7D1: Ast to Rows, Sum of Workload, sorted largest first.
func w7d1(t Table) computed {
	total := totalBy(t, "Ast")
	return oneWay(sortedTotals(total), total, "Sum of Workload")
}

// W7D2: two renamed metrics per assistant, sorted by workload.
func w7d2(t Table) computed {
	total, count := totalBy(t, "Ast"), sessionsBy(t, "Ast")
	var rows []map[string]any
	for _, a := range sortedTotals(total) {
		rows = append(rows, map[string]any{"Row Labels": a, "Total Workload": total[a], "Sessions": count[a]})
	}
	rows = append(rows, map[string]any{"Row Labels": grandTotal, "Total Workload": sum(total), "Sessions": sum(count)})
	return computed{[]string{"Row Labels", "Total Workload", "Sessions"}, rows}
}

// W7D3: Group down, Lecturer Type across. Pivot items sort A to Z, so Forum
// comes before VC.
func w7d3(t Table) computed {
	grid := map[string]float64{}
	for _, r := range t.Rows {
		grid[str(r["Group"])+"|"+str(r["Lecturer Type"])] += num(r["Workload"])
	}
	return pivotGrid(keysSorted(totalBy(t, "Group")), []string{"Forum", "VC"},
		func(g, ty string) float64 { return grid[g+"|"+ty] })
}

var shiftCodes = []string{"M", "N", "P"}

// W7D4: the roster in long form, Day down, shift code across. A day off is
// not a shift, so it is not counted.
func w7d4(t Table) computed {
	return pivotGrid(weekdays, shiftCodes, func(day, code string) float64 {
		n := 0.0
		for _, r := range t.Rows {
			if str(r[day]) == code {
				n++
			}
		}
		return n
	})
}

// W7D5: a calculated field divides the summed inputs, so the grand total is
// 72 / 41 rather than the average of the rows above it.
func w7d5(t Table) computed {
	total, count := totalBy(t, "Ast"), sessionsBy(t, "Ast")
	cols := []string{"Row Labels", "Sum of Workload", "Sessions", "Avg per Session"}
	var rows []map[string]any
	for _, a := range keysSorted(total) {
		rows = append(rows, map[string]any{
			"Row Labels": a, "Sum of Workload": total[a], "Sessions": count[a],
			"Avg per Session": round4(total[a] / count[a]),
		})
	}
	rows = append(rows, map[string]any{
		"Row Labels": grandTotal, "Sum of Workload": sum(total), "Sessions": sum(count),
		"Avg per Session": round4(sum(total) / sum(count)),
	})
	return computed{cols, rows}
}

// --- week 8 ---------------------------------------------------------------

var dayBand = map[string]string{
	"Monday": "Early week", "Tuesday": "Early week", "Wednesday": "Early week",
	"Thursday": "Late week", "Friday": "Late week", "Saturday": "Late week",
}

// W8D1: days grouped into two bands. Grouping reshapes; the total stays put.
func w8d1(t Table) computed {
	total := map[string]float64{}
	for _, r := range t.Rows {
		total[dayBand[str(r["Day"])]] += num(r["Workload"])
	}
	return oneWay([]string{"Early week", "Late week"}, total, "Sum of Workload")
}

// W8D2: one field shown three ways. Pivot ranks are dense: two assistants
// tied first are both 1, and the next value down is 2.
func w8d2(t Table) computed {
	total := totalBy(t, "Ast")
	all := sum(total)
	cols := []string{"Row Labels", "No Calculation", "% of Column Total", "Rank"}
	var rows []map[string]any
	rank, prev := 0, math.Inf(1)
	for _, a := range sortedTotals(total) {
		if total[a] < prev {
			rank++
			prev = total[a]
		}
		rows = append(rows, map[string]any{
			"Row Labels": a, "No Calculation": total[a],
			"% of Column Total": round4(total[a] / all), "Rank": float64(rank),
		})
	}
	rows = append(rows, map[string]any{
		"Row Labels": grandTotal, "No Calculation": all, "% of Column Total": 1.0, "Rank": "",
	})
	return computed{cols, rows}
}

const w8d3Group, w8d3Type = "G2", "VC"

// W8D3: the assistant pivot under two slicers. Assistants with nothing left
// drop out of the pivot rather than showing 0.
func w8d3(t Table) computed {
	total := map[string]float64{}
	for _, r := range t.Rows {
		if str(r["Group"]) == w8d3Group && str(r["Lecturer Type"]) == w8d3Type {
			total[str(r["Ast"])] += num(r["Workload"])
		}
	}
	return oneWay(keysSorted(total), total, "Sum of Workload")
}

const w8d4Group = "G3"

// W8D4: two pivots on one Group slicer: different rows, the same total.
func w8d4(t Table) computed {
	asts, days := map[string]float64{}, map[string]float64{}
	for _, r := range t.Rows {
		if str(r["Group"]) != w8d4Group {
			continue
		}
		asts[str(r["Ast"])] += num(r["Workload"])
		days[str(r["Day"])] += num(r["Workload"])
	}
	return computed{[]string{"Pivot", "Rows shown", grandTotal}, []map[string]any{
		{"Pivot": "Workload by assistant", "Rows shown": float64(len(asts)), grandTotal: sum(asts)},
		{"Pivot": "Workload by day", "Rows shown": float64(len(days)), grandTotal: sum(days)},
	}}
}

// W8D5: the roster unpivoted to one row per person per day, counted by cohort
// and code. Days off are dropped, so a short row total means days off.
func w8d5(t Table) computed {
	grid := map[string]float64{}
	for _, r := range t.Rows {
		for _, d := range weekdays {
			if code := str(r[d]); code != "" {
				grid[str(r["Gen"])+"|"+code]++
			}
		}
	}
	return pivotGrid(uniqueInOrder(t, "Gen"), shiftCodes,
		func(gen, code string) float64 { return grid[gen+"|"+code] })
}

const w8d6Group = "G1"

// W8D6: the controlled tab, unfiltered and under a G1 slicer. The share
// re-bases to whatever the slicer leaves.
func w8d6(t Table) computed {
	type view struct {
		early, late float64
		byAst       map[string]float64
	}
	build := func(keep func(map[string]any) bool) view {
		v := view{byAst: map[string]float64{}}
		for _, r := range t.Rows {
			if !keep(r) {
				continue
			}
			w := num(r["Workload"])
			if dayBand[str(r["Day"])] == "Early week" {
				v.early += w
			} else {
				v.late += w
			}
			v.byAst[str(r["Ast"])] += w
		}
		return v
	}
	top := func(v view) string {
		a := sortedTotals(v.byAst)[0]
		return fmt.Sprintf("%s, %.1f%%", a, 100*v.byAst[a]/(v.early+v.late))
	}
	all := build(func(map[string]any) bool { return true })
	g1 := build(func(r map[string]any) bool { return str(r["Group"]) == w8d6Group })

	return computed{[]string{"Pivot", "All groups", "G1 only"}, []map[string]any{
		{"Pivot": "Early week workload", "All groups": all.early, "G1 only": g1.early},
		{"Pivot": "Late week workload", "All groups": all.late, "G1 only": g1.late},
		{"Pivot": "Largest share", "All groups": top(all), "G1 only": top(g1)},
	}}
}

// --- week 9 ---------------------------------------------------------------

const w9d1Type = "Forum"

// W9D1: the pivot behind the chart, under a Lecturer Type slicer.
func w9d1(t Table) computed {
	total := map[string]float64{}
	for _, r := range t.Rows {
		if str(r["Lecturer Type"]) == w9d1Type {
			total[str(r["Group"])] += num(r["Workload"])
		}
	}
	return oneWay(keysSorted(total), total, "Sum of Workload")
}

// W9D2: teaching load by cohort and by the shift that assistant works on the
// session's day. A session on a day off has no shift, labelled "off".
func w9d2(t Table) computed {
	roster := rosterByInisial()
	grid := map[string]float64{}
	gens, shifts := map[string]bool{}, map[string]bool{}
	for _, r := range t.Rows {
		gen, shift := "unknown", "unknown"
		if p, ok := roster[str(r["Ast"])]; ok {
			gen = str(p["Gen"])
			if shift = str(p[str(r["Day"])[:3]]); shift == "" {
				shift = "off"
			}
		}
		gens[gen], shifts[shift] = true, true
		grid[gen+"|"+shift] += num(r["Workload"])
	}
	cols := append([]string{}, shiftCodes...)
	for _, extra := range []string{"off", "unknown"} {
		if shifts[extra] {
			cols = append(cols, extra)
		}
	}
	return pivotGrid(keysSortedBool(gens), cols,
		func(gen, shift string) float64 { return grid[gen+"|"+shift] })
}

func workloadByGen(t Table) map[string]float64 {
	roster := rosterByInisial()
	total := map[string]float64{}
	for _, r := range t.Rows {
		gen := "unknown"
		if p, ok := roster[str(r["Ast"])]; ok {
			gen = str(p["Gen"])
		}
		total[gen] += num(r["Workload"])
	}
	return total
}

const w9d3Gen = "24-2"

// W9D3: the baseline the dashboard must return to, beside one Gen slicer.
func w9d3(t Table) computed {
	roster := rosterByInisial()
	measure := func(keep func(map[string]any) bool) (sessions, workload, people float64) {
		seen := map[string]bool{}
		for _, r := range t.Rows {
			if !keep(r) {
				continue
			}
			sessions++
			workload += num(r["Workload"])
			seen[str(r["Ast"])] = true
		}
		return sessions, workload, float64(len(seen))
	}
	s1, w1, p1 := measure(func(map[string]any) bool { return true })
	s2, w2, p2 := measure(func(r map[string]any) bool {
		return str(roster[str(r["Ast"])]["Gen"]) == w9d3Gen
	})
	col := "Gen " + w9d3Gen
	return computed{[]string{"Measure", "All", col}, []map[string]any{
		{"Measure": "Sessions", "All": s1, col: s2},
		{"Measure": "Workload", "All": w1, col: w2},
		{"Measure": "Assistants", "All": p1, col: p2},
	}}
}

// W9D4: the cohort answer through a relationship instead of a copied column.
func w9d4(t Table) computed {
	total := workloadByGen(t)
	return oneWay(keysSorted(total), total, "Sum of Workload")
}

// W9D5: the four questions the capstone must answer.
func w9d5(t Table) computed {
	total := totalBy(t, "Ast")
	names := sortedTotals(total)
	var top []string
	for _, a := range names {
		if total[a] == total[names[0]] {
			top = append(top, a)
		}
	}
	values := make([]float64, 0, len(total))
	for _, v := range total {
		values = append(values, v)
	}
	sort.Float64s(values)
	median := values[len(values)/2]

	day := totalBy(t, "Day")
	days := append([]string{}, w3d5Days...)
	sort.SliceStable(days, func(i, j int) bool { return day[days[i]] < day[days[j]] })

	lt := totalBy(t, "Lecturer Type")
	all := sum(lt)

	gen := workloadByGen(t)
	var cohorts []string
	for _, g := range keysSorted(gen) {
		cohorts = append(cohorts, fmt.Sprintf("%s carries %s", g, one(gen[g])))
	}

	return computed{[]string{"Question", "Answer"}, []map[string]any{
		{"Question": "Who is overloaded?", "Answer": fmt.Sprintf("%s, %s each, against a median of %s.",
			joinAnd(top), one(total[top[0]]), one(median))},
		{"Question": "Which day is thin?", "Answer": fmt.Sprintf("%s at %s, then %s at %s.",
			days[0], one(day[days[0]]), days[1], one(day[days[1]]))},
		{"Question": "VC against Forum", "Answer": fmt.Sprintf("VC carries %s of %s (%.1f%%), Forum %s.",
			one(lt["VC"]), one(all), 100*lt["VC"]/all, one(lt["Forum"]))},
		{"Question": "Load across cohorts", "Answer": joinAnd(cohorts) + "."},
	}}
}
