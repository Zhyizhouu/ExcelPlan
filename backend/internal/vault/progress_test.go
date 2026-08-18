package vault

import "testing"

func note(phase string, week int, slug string, complete bool) Note {
	return Note{Phase: phase, Week: week, Slug: slug, Complete: complete}
}

func TestRoll(t *testing.T) {
	notes := []Note{
		note("0", 1, "W1D1", true),
		note("0", 1, "W1D2", true),
		note("0", 1, "W1D3", false),
		note("1", 2, "W2D1", false),
	}
	phases := map[string]Phase{
		"0": {Key: "0", Name: "Calibration", WeekRange: "Week 1", Order: 0},
	}

	got := Roll(notes, phases)

	t.Run("totals roll all the way up", func(t *testing.T) {
		if got.Completed != 2 || got.Total != 4 {
			t.Fatalf("got %d/%d, want 2/4", got.Completed, got.Total)
		}
	})

	t.Run("phase 0 carries its name from the hub note", func(t *testing.T) {
		if got.Phases[0].Name != "Calibration" || got.Phases[0].Completed != 2 {
			t.Fatalf("phase 0 = %+v", got.Phases[0])
		}
	})

	// The whole reason ReadPhases and ReadAll are separate reads: a phase can
	// have day notes before the hub note is updated to name it. Dropping those
	// notes would make them look deleted rather than merely unlabelled.
	t.Run("a phase absent from the hub note still appears, unnamed", func(t *testing.T) {
		if len(got.Phases) != 2 {
			t.Fatalf("got %d phases, want 2", len(got.Phases))
		}
		if got.Phases[1].Key != "1" || got.Phases[1].Name != "" {
			t.Fatalf("phase 1 = %+v", got.Phases[1])
		}
	})

	t.Run("NextUp is the first incomplete note in plan order", func(t *testing.T) {
		if got.NextUp == nil || got.NextUp.Slug != "W1D3" {
			t.Fatalf("NextUp = %+v, want W1D3", got.NextUp)
		}
	})
}

// The merged "2-3" key is the real vault's shape, not a hypothetical: 25
// notes use it while the hub note names Phase 2 and Phase 3 separately.
func TestRoll_MergedPhaseKey(t *testing.T) {
	notes := []Note{
		note("2-3", 5, "W5D1", true),
		note("2-3", 7, "W7D1", false),
	}
	headers := map[string]Phase{
		"2": {Key: "2", Name: "Charts & Dashboards", WeekRange: "Weeks 5-6", Order: 2},
		"3": {Key: "3", Name: "Pivot Tables & Slicers", WeekRange: "Weeks 7-9", Order: 3},
	}

	got := Roll(notes, headers)

	if len(got.Phases) != 1 {
		t.Fatalf("got %d phases, want the two halves rolled into one", len(got.Phases))
	}
	p := got.Phases[0]
	if p.Name != "Charts & Dashboards · Pivot Tables & Slicers" {
		t.Fatalf("name = %q, want both halves joined", p.Name)
	}
	if p.WeekRange != "Weeks 5-9" {
		t.Fatalf("weeks = %q, want the spanned range", p.WeekRange)
	}
	if p.Order != 2 {
		t.Fatalf("order = %d, want 2 so it sorts between phases 1 and 4", p.Order)
	}
}

func TestRoll_EverythingComplete(t *testing.T) {
	notes := []Note{note("0", 1, "W1D1", true), note("0", 1, "W1D2", true)}
	got := Roll(notes, nil)

	if got.NextUp != nil {
		t.Fatalf("NextUp = %+v, want nil once everything is done", got.NextUp)
	}
	if got.Completed != got.Total {
		t.Fatalf("completed %d != total %d", got.Completed, got.Total)
	}
}

func TestRoll_Empty(t *testing.T) {
	got := Roll(nil, nil)
	if got.Total != 0 || got.NextUp != nil || len(got.Phases) != 0 {
		t.Fatalf("got %+v, want a clean zero value", got)
	}
}
