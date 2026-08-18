// Package progresslog stamps the one thing the vault's checkbox cannot say on
// its own: when.
//
// A day note's checkbox is a bare boolean — checked or not — with no record of
// which date it was ticked on. That is enough to render "62 of 110 done," but
// not "did you show up today," which a streak needs. Rather than write a
// timestamp into the vault's own files — a second thing to keep in sync with
// Obsidian, and a file format this app does not own — completion dates live
// here, in a small store this app does own, built by watching the vault for
// checkboxes flipping from unchecked to checked.
package progresslog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Zhyizhouu/excelplan/internal/vault"
)

// Log is a slug -> the date it was first observed complete.
//
// Kept as a plain map, not a list of events: a completion has exactly one
// date that matters for a streak — when it happened — and re-ticking an
// already-ticked box (which the vault cannot even distinguish from "was
// always ticked") has nothing further to record.
type Log struct {
	path    string
	entries map[string]time.Time // slug -> completed date (day precision)
}

// Open reads path if it exists, or starts empty if this is the first run.
func Open(path string) (*Log, error) {
	l := &Log{path: path, entries: map[string]time.Time{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	for slug, day := range stored {
		if t, err := time.Parse("2006-01-02", day); err == nil {
			l.entries[slug] = t
		}
	}
	return l, nil
}

// Observe reconciles the log against the vault's current state: a note newly
// complete is stamped with today; a note no longer complete (unchecked back
// in Obsidian) has its stamp removed, because the log's job is to say when the
// *current* completion happened, and an undone box was not completed.
//
// Returns whether anything changed, so the caller only writes to disk when
// there is something new to write.
func (l *Log) Observe(notes []vault.Note, now time.Time) bool {
	today := now.Truncate(24 * time.Hour)
	seen := make(map[string]bool, len(notes))
	changed := false

	for _, n := range notes {
		seen[n.Slug] = true
		_, logged := l.entries[n.Slug]
		switch {
		case n.Complete && !logged:
			l.entries[n.Slug] = today
			changed = true
		case !n.Complete && logged:
			delete(l.entries, n.Slug)
			changed = true
		}
	}
	// A note that vanished from the vault entirely (renamed, deleted) should
	// not keep contributing to a streak nothing can show anymore.
	for slug := range l.entries {
		if !seen[slug] {
			delete(l.entries, slug)
			changed = true
		}
	}
	return changed
}

// Save writes the log to disk. Errors are the caller's to decide about — an
// unwritable log should not stop progress from being reported, only stop that
// day's completion from being remembered for the streak.
func (l *Log) Save() error {
	stored := make(map[string]string, len(l.entries))
	for slug, t := range l.entries {
		stored[slug] = t.Format("2006-01-02")
	}
	raw, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(l.path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(l.path, raw, 0o644)
}

// Streak is today's read on momentum: the current run and the best one ever.
type Streak struct {
	Current int `json:"current"`
	Longest int `json:"longest"`
	// LastActive is nil if nothing has ever been logged.
	LastActive *time.Time `json:"lastActive,omitempty"`
}

// Compute derives a streak from the distinct calendar days that have at least
// one completion — not from consecutive slugs, which would count "did five
// days in one sitting" as five days of momentum instead of one.
//
// The current streak stays alive through today even if nothing is checked
// yet: study happens at whatever hour it happens, and a streak that resets at
// midnight punishes an evening session for not having happened by breakfast.
// It breaks only once a full day has passed with nothing logged.
func Compute(l *Log, now time.Time) Streak {
	if len(l.entries) == 0 {
		return Streak{}
	}

	days := make(map[time.Time]bool, len(l.entries))
	for _, t := range l.entries {
		days[t] = true
	}
	sorted := make([]time.Time, 0, len(days))
	for d := range days {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	// longest tracks the best run seen anywhere in the history; trailing tracks
	// the run ending at the very last day, which becomes the current streak
	// below if that last day is recent enough to still be "alive."
	longest, trailing := 1, 1
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Sub(sorted[i-1]) == 24*time.Hour {
			trailing++
		} else {
			trailing = 1
		}
		if trailing > longest {
			longest = trailing
		}
	}

	last := sorted[len(sorted)-1]
	today := now.Truncate(24 * time.Hour)

	current := 0
	if today.Sub(last) <= 24*time.Hour { // active today or yesterday — still alive
		current = trailing
	}

	return Streak{Current: current, Longest: longest, LastActive: &last}
}
