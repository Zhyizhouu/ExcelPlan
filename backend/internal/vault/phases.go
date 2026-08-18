package vault

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// phaseHeaderRE matches "## Phase 0 - Calibration & Speed  (Week 1)" and
// "## Phase 1 - Formula Fluency & Data Management  (Weeks 2-4)" — the one
// place in the vault a phase's name is written down. Day-note frontmatter
// only carries the number.
var phaseHeaderRE = regexp.MustCompile(`^##\s*Phase\s+(\d+)\s*-\s*(.+?)\s*\((Weeks?\s+[\d-]+)\)\s*$`)

// ReadPhases parses the phase names and week ranges out of the hub note
// (Excel_Mastery.md), keyed by the single number in each header.
//
// The hub note always names phases individually, even the ones the day notes
// merge — there are separate "## Phase 2" and "## Phase 3" headers although
// 25 notes are filed as `phase: 2-3`. Reconciling that is Roll's job, since
// only it sees which keys the notes actually use.
func ReadPhases(hubNotePath string) (map[string]Phase, error) {
	f, err := os.Open(hubNotePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	phases := map[string]Phase{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		m := phaseHeaderRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		phases[m[1]] = Phase{
			Key:       m[1],
			Name:      m[2],
			WeekRange: m[3],
			Order:     phaseOrder(m[1]),
		}
	}
	return phases, sc.Err()
}

// describe builds the display Phase for a key the notes actually use, joining
// the halves of a merged key.
//
// "2-3" becomes "Charts & Dashboards · Pivot Tables & Slicers", spanning
// "Weeks 5-6" and "Weeks 7-9" into "Weeks 5-9". A key with no matching header
// still returns a usable Phase — unnamed, but present and correctly ordered,
// because a phase with notes and no header is a real mid-edit state of the hub
// note and hiding it would look like the notes went missing.
func describe(key string, headers map[string]Phase) Phase {
	out := Phase{Key: key, Order: phaseOrder(key)}

	var names []string
	var firstWeeks, lastWeeks string
	for _, part := range phaseParts(key) {
		h, ok := headers[part]
		if !ok {
			continue
		}
		names = append(names, h.Name)
		if firstWeeks == "" {
			firstWeeks = h.WeekRange
		}
		lastWeeks = h.WeekRange
	}

	out.Name = strings.Join(names, " · ")
	out.WeekRange = spanWeeks(firstWeeks, lastWeeks)
	return out
}

// weekNumRE pulls the numbers out of "Weeks 5-6" or "Week 1".
var weekNumRE = regexp.MustCompile(`\d+`)

// spanWeeks joins two week ranges into the range they cover together, so a
// merged phase reads "Weeks 5-9" rather than "Weeks 5-6 · Weeks 7-9".
func spanWeeks(first, last string) string {
	if first == "" {
		return ""
	}
	if first == last {
		return first
	}
	lo := weekNumRE.FindAllString(first, -1)
	hi := weekNumRE.FindAllString(last, -1)
	if len(lo) == 0 || len(hi) == 0 {
		return first
	}
	return "Weeks " + lo[0] + "-" + hi[len(hi)-1]
}
