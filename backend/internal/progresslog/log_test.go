package progresslog

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Zhyizhouu/excelplan/internal/vault"
)

func day(offset int) time.Time {
	return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func openEmpty(t *testing.T) *Log {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "log.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestObserve(t *testing.T) {
	t.Run("a newly complete note is stamped with today", func(t *testing.T) {
		l := openEmpty(t)
		changed := l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(0))
		if !changed {
			t.Fatal("expected a change")
		}
		if _, ok := l.entries["W1D1"]; !ok {
			t.Fatal("W1D1 was not recorded")
		}
	})

	t.Run("re-observing an already-logged completion is a no-op", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(0))
		changed := l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(1))
		if changed {
			t.Fatal("re-observing the same completion should not report a change")
		}
		if got := l.entries["W1D1"]; !got.Equal(day(0).Truncate(24 * time.Hour)) {
			t.Fatalf("stamp moved to %v, want it to stay at day 0", got)
		}
	})

	t.Run("unchecking a box in Obsidian removes its stamp", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(0))
		changed := l.Observe([]vault.Note{{Slug: "W1D1", Complete: false}}, day(1))
		if !changed {
			t.Fatal("expected a change")
		}
		if _, ok := l.entries["W1D1"]; ok {
			t.Fatal("W1D1 stamp should have been removed")
		}
	})

	t.Run("a note that vanishes from the vault is forgotten too", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(0))
		l.Observe([]vault.Note{}, day(1)) // W1D1's file no longer exists
		if _, ok := l.entries["W1D1"]; ok {
			t.Fatal("a deleted note should not keep contributing to the streak")
		}
	})
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "log.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	l.Observe([]vault.Note{{Slug: "W1D1", Complete: true}}, day(0))
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.entries["W1D1"]; !ok {
		t.Fatal("W1D1 did not survive a save/reload round trip")
	}
}

func TestComputeStreak(t *testing.T) {
	t.Run("nothing logged is a zero streak, not an error", func(t *testing.T) {
		got := Compute(openEmpty(t), day(0))
		if got.Current != 0 || got.Longest != 0 || got.LastActive != nil {
			t.Fatalf("got %+v, want a clean zero value", got)
		}
	})

	// Observe always receives the *whole* vault, so these build the note list
	// up cumulatively. Passing only the newly-ticked note would be read as
	// every other note having been deleted — which is correct behaviour, and
	// exactly what the "vanishes from the vault" case above pins down.
	t.Run("three consecutive days, checked today, is a live streak of three", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{
			{Slug: "A", Complete: true},
			{Slug: "B", Complete: false},
			{Slug: "C", Complete: false},
		}, day(-2))
		l.Observe([]vault.Note{
			{Slug: "A", Complete: true},
			{Slug: "B", Complete: true},
			{Slug: "C", Complete: false},
		}, day(-1))
		l.Observe([]vault.Note{
			{Slug: "A", Complete: true},
			{Slug: "B", Complete: true},
			{Slug: "C", Complete: true},
		}, day(0))

		got := Compute(l, day(0))
		if got.Current != 3 || got.Longest != 3 {
			t.Fatalf("got current=%d longest=%d, want 3/3", got.Current, got.Longest)
		}
	})

	// The forgiveness rule: nothing logged yet today should not zero out a
	// streak from yesterday. Study happens at whatever hour it happens.
	t.Run("nothing today yet, but yesterday was logged, keeps the streak alive", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "A", Complete: true}}, day(-1))

		got := Compute(l, day(0))
		if got.Current != 1 {
			t.Fatalf("current = %d, want 1 (still alive through today)", got.Current)
		}
	})

	t.Run("a full day gone by with nothing logged breaks the streak", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "A", Complete: true}}, day(-3))

		got := Compute(l, day(0)) // two clear days of silence
		if got.Current != 0 {
			t.Fatalf("current = %d, want 0 (streak should be broken)", got.Current)
		}
		if got.Longest != 1 {
			t.Fatalf("longest = %d, want 1 (history is not erased by breaking)", got.Longest)
		}
	})

	t.Run("a gap in the middle resets the run but keeps the best one", func(t *testing.T) {
		l := openEmpty(t)
		all := func(done ...string) []vault.Note {
			set := map[string]bool{}
			for _, s := range done {
				set[s] = true
			}
			var out []vault.Note
			for _, s := range []string{"A", "B", "C", "D"} {
				out = append(out, vault.Note{Slug: s, Complete: set[s]})
			}
			return out
		}
		l.Observe(all("A"), day(-10))
		l.Observe(all("A", "B"), day(-9))
		l.Observe(all("A", "B", "C"), day(-8))
		// gap: nothing new on day -7 through -1
		l.Observe(all("A", "B", "C", "D"), day(0))

		got := Compute(l, day(0))
		if got.Longest != 3 {
			t.Fatalf("longest = %d, want 3 (the earlier three-day run)", got.Longest)
		}
		if got.Current != 1 {
			t.Fatalf("current = %d, want 1 (today starts a new run)", got.Current)
		}
	})

	t.Run("two completions on the same day count as one day of momentum", func(t *testing.T) {
		l := openEmpty(t)
		l.Observe([]vault.Note{{Slug: "A", Complete: true}, {Slug: "B", Complete: true}}, day(0))

		got := Compute(l, day(0))
		if got.Current != 1 {
			t.Fatalf("current = %d, want 1 (one day, not two)", got.Current)
		}
	})
}
