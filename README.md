# ExcelPlan

A progress tracker for the **Excel Mastery** study plan — 110 daily cases over
~22 weeks — read straight out of the Obsidian vault the plan already lives in.

The vault stays the source of truth. Cases are worked and ticked in Obsidian;
this app reads those files, ticks the same checkbox, and hands each case to
Excel. There is no second copy of the plan to keep in sync.

---

## Why I built this

I'm working toward a promotion where advanced Excel is not a nice-to-have on a
CV — it's the bar. Not formatting a table, but modelling a scenario, cleaning
data nobody else wants to touch, and building something a colleague can open
six months later and still trust.

Rather than wait for that skill to arrive on the job, I decided to close the
gap deliberately. I wrote out a 110-case, 22-week curriculum in Obsidian —
five cases a week, each one a small realistic problem rather than a tutorial to
follow along with — and started on a Monday.

The plan was the easy part. Everyone can write a plan on a Monday.

What actually decides whether a self-directed curriculum survives week three is
much smaller than motivation. It's the twenty seconds between sitting down and
starting: *which case was I on? where's the data? do I have to build the
spreadsheet before I can begin?* Every one of those is an exit, and I'd taken
that exit on plans before.

So I built the thing that removes them. ExcelPlan opens on the next case —
already chosen, no deciding required — and one click writes that case's
workbook into the vault and opens it in Excel, brief and data already laid out.
The gap between intention and work is now a single click, which is roughly the
smallest I know how to make it.

**A few principles I held to, and would defend in a code review:**

- **One source of truth, no exceptions.** The plan lives in Obsidian and this
  app never keeps a competing copy. A tracker that disagrees with your notes is
  worse than no tracker, because now you have to reconcile two records.
- **Every number is arithmetic you can check by hand.** The projected finish
  date is `remaining ÷ your weekly rate` — no smoothing, no model. A projection
  nobody believes is worse than none at all.
- **Motivation design, deliberately restrained.** No streak-loss warnings, no
  manufactured urgency, no confetti per checkbox. The one thing that celebrates
  is finishing a phase, once.
- **Refuse the unrecoverable action.** A workbook already in the vault is never
  written over, and there is no force flag to make it — that file is where the
  answers get typed. Opening a case you have already started hands you back
  exactly what you left.

The honest summary: I set out to learn Excel and ended up shipping a small
full-stack tool to keep myself learning Excel. Both were worth the time — but
the plan is still the point, and this is scaffolding around it.

---

## Running it

Two processes, both local:

```bash
# backend — reads the vault, serves progress on :8090
cd backend
go run ./cmd/api

# frontend — Vite dev server on :5173
cd frontend
npm install
npm run dev
```

Then open http://localhost:5173.

### Configuration

All optional; the defaults point at the real vault on this machine.

| Variable | Default |
|---|---|
| `EXCELPLAN_VAULT_ROOT` | `D:\Abner\Obsidian\Fred\Excel Mastery` |
| `EXCELPLAN_HUB_NOTE` | `<vault root>\Excel_Mastery.md` |
| `EXCELPLAN_LOG_PATH` | `progress-log.json` |
| `EXCELPLAN_ADDR` | `:8090` |
| `VITE_API_BASE` (frontend) | `http://localhost:8090` |

---

## Layout

```
backend/
  cmd/api/            the one binary: config from env, then serve
  internal/
    vault/            read the notes, parse them, tick the checkbox
    workbook/         build the three-sheet .xlsx
    datasets/         the input tables and worked examples, as JSON
    progresslog/      completion dates and streaks (see below)
    openfile/         hand a saved workbook to Excel
    httpapi/          the routes
frontend/
  src/screens/        Dashboard, Phases, CaseDetail, SignIn
  src/components/     the small shared kit
  src/api/progress.ts every call the app makes
```

A Go backend rather than reading the vault from the browser, because a page
cannot read a folder off disk, write to a note, or open Excel — and all three
are the point.

---

## How progress is read

Each day note ends with one checkbox:

```markdown
- [ ] W1D1 complete
```

That single box is the whole progress model. Weeks roll up from days, phases
from weeks, and the plan from phases.

Phase names and week ranges come from the headers in `Excel_Mastery.md`, since
the day notes carry only a phase number.

### Two things the real vault does that the parser has to handle

**Phases 2 and 3 share one folder.** 25 notes carry `phase: 2-3` while the hub
note names Phase 2 and Phase 3 separately. The phase key is therefore a string,
not a number, and a merged key takes both halves' names and the span of their
week ranges ("Weeks 5-9").

**Frontmatter is hand-written, so it is not always valid YAML.** One note reads
`title: W22D2 - Solver: balance the roster` — an unquoted second colon, which a
strict YAML parser rejects outright. Every field this app reads is a flat
scalar, so the frontmatter is scanned line by line instead. A tracker that
falls over because a title contains a colon is not much of a tracker.

### Where the completion dates come from

The checkbox is a bare boolean: it says *whether* a case is done, never *when*.
Streaks need dates, so the backend keeps its own small log
(`progress-log.json`), stamping a case the first time it is *observed*
complete, and clearing the stamp if the box is later unticked.

Nothing is ever written back into the vault beyond that one checkbox. The trade
is that a case ticked while the backend is not running is dated whenever it
next runs — the alternative was writing timestamps into note files this app
does not own.

### Ticking from either side

Obsidian and ExcelPlan edit the same file, so whichever was touched last is
simply what is true. In the app a tick is optimistic — the box flips
immediately, then the server's recomputed plan replaces the guess, and a
failure rolls it back and says which case was lost. Completion is read from
that one shared plan on every screen, so ticking a case on its detail page
moves the dashboard, the week, the phase and the streak at the same moment.

The other direction is covered by refetching on window focus: tick a box in
Obsidian, alt-tab back, and it's already there. No polling loop, no socket, and
it fires exactly when the answer might have changed.

---

## The motivation design

Deliberately restrained. Every number shown is arithmetic on real data:

- **Next up** is the largest thing on the dashboard, because choosing what to
  do is the step that gets skipped a day over. It is already chosen.
- **The finish date** is `remaining ÷ your weekly rate`, defaulting to the
  plan's own stated cadence of five a week. Checkable by hand — a projection
  nobody believes is worse than none.
- **The pace card** runs that arithmetic from both ends. *From a pace* takes
  cases-a-day and days-a-week and returns a date; *from a date* takes the
  deadline and returns the pace it would cost. The second is the one that
  earns its place: a projected date you dislike tells you nothing about what
  to change, whereas "that date needs 3.6 a study day instead of 1" is a
  decision — often the decision to move the date, which is a legitimate answer.

  The rate is two inputs rather than one because "a case a day" is ambiguous
  in exactly the way that matters: five a week and seven a week are six
  calendar weeks apart over 110 cases. Splitting it into cases-per-sitting and
  sittings-per-week makes the assumption visible instead of letting the
  arithmetic pick one silently. The default — one a day, five days a week —
  reproduces the old fixed projection exactly, so the feature is opt-in rather
  than a silent recalculation of a number you had already learned to trust.

  Required paces are not rounded down to look achievable, and the projection
  charges a whole sitting for a partial one, because you cannot do four fifths
  of a study session.
- **The streak** counts distinct days with at least one completion, not
  consecutive cases, so five cases in one sitting is one day of momentum. It
  survives through today rather than resetting at midnight, because studying at
  10pm should not be punished for not having happened by breakfast.
- **Phase completion** is the only thing that celebrates, once, and never
  again for the same phase. A checkbox is too small to interrupt for; the whole
  110-case plan is too far off to feel reachable.

No streak-loss warnings, no artificial urgency, no confetti per checkbox.

---

## Getting a case into Excel

Each case detail page offers four routes out.

**Open in Excel** is the primary one, and the one the habit runs on. It puts a
three-sheet workbook at `Phase <x>/<Case> - Answer.xlsx`, beside its note, and
opens it:

- **Brief** — the case as written
- **Data** — the input table
- **Expected** — the answer, on its own sheet so opening the file does not give
  it away

**Save to vault** does the same without the launch, for stocking up a few cases
ahead of a session you are not starting yet. **Download .xlsx** and **Copy as
CSV** are for working somewhere else — another machine, or a sheet already
open.

Three deliberate constraints:

**An existing workbook is never written to.** That file is where the answers
get typed, and replacing it is the single unrecoverable thing this app could
do. So the button that opens a case you have already started opens *your*
workbook, answers and all — it does not rebuild it, and there is no force flag
anywhere to make it. The guarantee is `O_EXCL` on the create, not a check-then-
write that a race could slip through.

Nor is a second visit an error. Both buttons mean "get me into this case", and
after the first one that is a file which already exists; the old 409 made the
safe path feel like a failure. It reports `created: false` instead, and the
page says the file was left untouched.

**Opening is the server's job.** The file is on the same machine as the
backend, and a browser has no way to hand a local path to Excel. A failed
launch is not a failed save — the page says the workbook is there and to open
it by hand, because reporting failure would send you looking for a file you
already have.

**The Data sheet is a plain styled range, not an Excel Table.** W2D1's whole
exercise is converting a range with Ctrl+T; shipping a Table pre-made would
complete that case on your behalf.

The `.xlsx` extension also keeps these files invisible to the note parser,
which only ever globs `*.md` — so answer workbooks can live beside their cases
without being counted as cases.

---

## Tests

```bash
cd backend  && go test ./...
cd frontend && npm test && npm run check
```

The backend suite includes tests that run against the **real vault** and skip
cleanly when it is absent — they assert the actual 110-note count, the merged
phase, and correct ordering, so a change to the real notes fails a test before
it surfaces as a wrong number on screen.

---

## What it deliberately does not do

No accounts, no database, no deploy target, no sync. The sign-in screen is
cosmetic and says so in its own source — this reads one person's study notes
off their own disk, and dressing that up as authentication would only invite
someone to later put something real behind it.
