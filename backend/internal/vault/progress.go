package vault

import "sort"

// WeekProgress is one week's line in the plan: how many of its five days are
// checked.
type WeekProgress struct {
	Phase     string `json:"phase"`
	Week      int    `json:"week"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Notes     []Note `json:"notes"`
}

// PhaseProgress rolls a phase's weeks up into one figure, and carries the
// phase's name and week range for display — the two things day-note
// frontmatter alone cannot supply.
type PhaseProgress struct {
	Phase
	Completed int            `json:"completed"`
	Total     int            `json:"total"`
	Weeks     []WeekProgress `json:"weeks"`
}

// Progress is the whole plan as one read: every phase, and the single day to
// do next.
type Progress struct {
	Phases    []PhaseProgress
	Completed int
	Total     int
	// NextUp is the first incomplete note in plan order — phase, then week,
	// then slug — which is also the order ReadAll already returns. Nil once
	// everything is checked.
	//
	// This exists so the dashboard can answer "what do I do today" with one
	// field instead of the UI re-deriving "first false in a sorted list" from
	// a full note dump. That is exactly the kind of decision that drifts if
	// two screens compute it independently.
	NextUp *Note
}

// Roll turns a flat, sorted note list (as ReadAll returns it) and the hub
// note's phase headers (as ReadPhases returns them) into the nested shape
// every screen renders directly.
func Roll(notes []Note, headers map[string]Phase) Progress {
	var out Progress
	phaseIdx := make(map[string]int)
	type weekKey struct {
		phase string
		week  int
	}
	weekIdx := make(map[weekKey]int)

	for _, note := range notes {
		pi, ok := phaseIdx[note.Phase]
		if !ok {
			out.Phases = append(out.Phases, PhaseProgress{Phase: describe(note.Phase, headers)})
			pi = len(out.Phases) - 1
			phaseIdx[note.Phase] = pi
		}

		wk := weekKey{note.Phase, note.Week}
		wi, ok := weekIdx[wk]
		if !ok {
			out.Phases[pi].Weeks = append(out.Phases[pi].Weeks,
				WeekProgress{Phase: note.Phase, Week: note.Week})
			wi = len(out.Phases[pi].Weeks) - 1
			weekIdx[wk] = wi
		}

		week := &out.Phases[pi].Weeks[wi]
		week.Notes = append(week.Notes, note)
		week.Total++
		out.Phases[pi].Total++
		out.Total++
		if note.Complete {
			week.Completed++
			out.Phases[pi].Completed++
			out.Completed++
		} else if out.NextUp == nil {
			n := note
			out.NextUp = &n
		}
	}

	// Notes arrive sorted, so phases are already in order — but sorting here
	// too means Roll does not silently depend on that, and an unsorted caller
	// gets a sensible plan rather than a scrambled one.
	sort.SliceStable(out.Phases, func(i, j int) bool {
		return out.Phases[i].Order < out.Phases[j].Order
	})
	return out
}
