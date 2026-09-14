package tutor

import (
	"fmt"
	"strings"

	"github.com/Zhyizhouu/excelplan/internal/datasets"
)

// SystemPrompt is the whole of the tutor's character.
//
// Written as one constant rather than assembled from settings because it is the
// feature: everything else here is plumbing that moves text to an API. Two
// rules in it are load-bearing and should not be softened without deciding to.
//
// The first is graduated help. The tutor knows the answer on the cases that
// have one, and will give it when asked — being stuck for forty minutes with no
// way forward is how a self-directed plan dies at week three. But it opens with
// the smallest nudge that could unblock, because a case solved for you in
// ninety seconds is a case you did not learn.
//
// The second is honesty about its own footing. Ten of the 127 cases carry a
// worked example; on the other hundred the tutor is deriving a good answer, not
// reciting the intended one, and it says so rather than sounding equally
// certain in both situations.
const SystemPrompt = `You are the case tutor inside ExcelPlan, a self-study Excel plan of 127
cases - five short ones a week plus a longer weekly build that ties them together. You are talking to one student — a working professional studying toward a
promotion that requires genuinely advanced Excel. Assume they are a capable
beginner: comfortable in a spreadsheet, new to formulas beyond SUM, and new to
VBA.

You are given the exact case they are on, its data, and what it asks for. Never
ask them to explain their own case back to you. Never open with "could you share
more about what you are trying to do" — you already know.

HOW MUCH TO GIVE AWAY
- Start with the smallest nudge that could unblock them: name the function to
  reach for, or point at the column they are ignoring.
- If they are still stuck, show the shape of the answer with the specifics left
  blank, e.g. =XLOOKUP(<what you are looking up>, <where>, <what to return>).
- If they ask outright for the answer, give it in full, then explain why it
  works. Never refuse, never stall, never make them ask twice.
- Never open with the finished formula unless they asked for it.

WHEN YOU ARE NOT SURE
Most cases give you the task but no stored solution. Where that is so, say your
approach is one good way rather than implying it is the intended one. Do not
invent a column that is not in the data you were given.

ON VBA
Some cases carry a VBA task. Before week 10 of the plan the student has not been
taught VBA yet, so those are read-and-run: explain what the code does line by
line, and do not expect them to write it. From week 10 they are learning to
write it, so guide rather than supply.
If VBA is the wrong tool for what they are asking, say so plainly and name the
formula, Power Query step, or LAMBDA that fits better. A macro a colleague
cannot maintain is worse than a formula they can read, and many workplaces block
macros outright.

HOW TO ANSWER
You reply in typed blocks. Use them properly:
- "table" whenever you are comparing two or more things, listing options, or
  mapping a keyword to what it does. Prefer a table to a paragraph. This is the
  single most useful thing you do — the student asked for tables, not prose.
- "code" for any formula or VBA. Set language to "excel" or "vba".
- "steps" for anything to do in order, in the real UI, with real key presses
  (Alt+F11, Ctrl+T) rather than vague directions.
- "text" for the connective explanation. Keep each one short. Two or three
  sentences, then show something.

Always use the student's real column names in examples, never Column A / Foo /
Bar. Concrete beats general every time.`

// CaseContext is everything the app knows about the case being asked about.
type CaseContext struct {
	Slug        string
	Title       string
	Phase       string
	Week        int
	Fields      []string // "Do: ..." etc, already flattened
	DatasetName string
	Columns     []string
	SampleRows  []string
	// The techniques the answer must genuinely use, and the input cells that
	// make that requirement bite. Sent so the tutor can hold the line rather
	// than cheerfully offering a shortcut that skips the whole point.
	Requires []string
	Controls []datasets.Control
	// Set only for the ten cases with a worked example. When absent the tutor
	// is told to say it is deriving rather than reciting.
	ExpectedFormula string
	ExpectedNote    string
}

// Render turns the context into the block that precedes every question.
//
// Plain labelled text, not JSON. The model reads this as briefing material, and
// prose survives a field being missing far better than a JSON object full of
// empty strings — which reads as "these values are blank" rather than "this
// case does not have those".
func (c CaseContext) Render() string {
	var b strings.Builder

	fmt.Fprintf(&b, "THE CASE THE STUDENT IS ON\n")
	fmt.Fprintf(&b, "%s — %s (Phase %s, week %d of 22)\n", c.Slug, c.Title, c.Phase, c.Week)

	for _, f := range c.Fields {
		fmt.Fprintf(&b, "%s\n", f)
	}

	if c.DatasetName != "" {
		fmt.Fprintf(&b, "\nTHE DATA\nSheet: %s\nColumns: %s\n",
			c.DatasetName, strings.Join(c.Columns, ", "))
		if len(c.SampleRows) > 0 {
			fmt.Fprintf(&b, "First rows:\n")
			for _, r := range c.SampleRows {
				fmt.Fprintf(&b, "  %s\n", r)
			}
		}
		fmt.Fprintf(&b, "The student's answer goes one column clear of this data.\n")
	}

	if len(c.Requires) > 0 {
		fmt.Fprintf(&b, "\nTECHNIQUES THE ANSWER MUST ACTUALLY USE\n%s\n",
			strings.Join(c.Requires, ", "))
		fmt.Fprintf(&b, "Do not help them around these. If they ask for a way that avoids "+
			"one, show it if they insist but say plainly which requirement it skips and "+
			"what they would not learn.\n")
	}

	if len(c.Controls) > 0 {
		fmt.Fprintf(&b, "\nINPUT CELLS THE ANSWER MUST REACT TO\n")
		for _, ctl := range c.Controls {
			fmt.Fprintf(&b, "  %s = %s — %s\n", ctl.Cell, ctl.Label, ctl.Value)
		}
		fmt.Fprintf(&b, "A hand-typed answer stops being right when these change. If they "+
			"paste something hardcoded, point at that rather than at the values.\n")
	}

	if c.ExpectedFormula != "" {
		fmt.Fprintf(&b, "\nTHE INTENDED SOLUTION (this case has a worked example)\n")
		fmt.Fprintf(&b, "%s\n", c.ExpectedFormula)
		if c.ExpectedNote != "" {
			fmt.Fprintf(&b, "What the result should show: %s\n", c.ExpectedNote)
		}
		fmt.Fprintf(&b, "This is the intended answer. Follow the graduated-help rule "+
			"before showing it.\n")
	} else {
		fmt.Fprintf(&b, "\nNo worked example is stored for this case. Any solution you give "+
			"is one good way, not the intended one — say so.\n")
	}

	return b.String()
}
