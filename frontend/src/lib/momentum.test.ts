import { beforeEach, describe, expect, it } from "vitest";
import type { Progress } from "../api/progress";
import {
  CADENCE_PER_WEEK,
  DEFAULT_PACE,
  completePhaseKeys,
  daysBetween,
  formatRate,
  loadPaceSettings,
  momentum,
  paceForDeadline,
  savePaceSettings,
  takeNewlyComplete,
} from "./momentum";

function progress(partial: Partial<Progress> = {}): Progress {
  return {
    completed: 0,
    total: 110,
    phases: [],
    streak: { current: 0, longest: 0 },
    ...partial,
  };
}

describe("momentum", () => {
  it("counts what is left and turns it into a percentage", () => {
    const m = momentum(progress({ completed: 55, total: 110 }));
    expect(m.remaining).toBe(55);
    expect(m.percent).toBe(50);
  });

  // The projection is the motivating number, so it has to be arithmetic
  // anybody could redo by hand: 55 left at 5 a week is 11 weeks, which is 77
  // days out. A date that cannot be checked is a date that stops being
  // believed.
  it("projects the finish date from the stated cadence, nothing else", () => {
    const now = new Date(2026, 7, 17);
    const m = momentum(progress({ completed: 55, total: 110 }), now);

    expect(m.weeksLeft).toBe(55 / CADENCE_PER_WEEK);
    const expected = new Date(2026, 7, 17);
    expected.setDate(expected.getDate() + 11 * 7);
    expect(m.finishDate?.toDateString()).toBe(expected.toDateString());
  });

  it("rounds a partial week up rather than down", () => {
    // 6 remaining is more than one week's work, so it must not read as one.
    const m = momentum(progress({ completed: 104, total: 110 }));
    expect(m.weeksLeft).toBe(2);
  });

  it("offers no finish date once the plan is done", () => {
    const m = momentum(progress({ completed: 110, total: 110 }));
    expect(m.remaining).toBe(0);
    expect(m.percent).toBe(100);
    expect(m.finishDate).toBeNull();
  });

  it("does not divide by zero on an empty plan", () => {
    const m = momentum(progress({ completed: 0, total: 0 }));
    expect(m.percent).toBe(0);
    expect(m.finishDate).toBeNull();
  });
});

describe("momentum at a chosen pace", () => {
  // The whole point of the default: someone who never touches the pace card
  // must see the number they saw before it existed.
  it("reproduces the plan's cadence when left at the default", () => {
    const now = new Date(2026, 7, 17);
    const p = progress({ completed: 55, total: 110 });
    expect(momentum(p, now, DEFAULT_PACE)).toEqual(momentum(p, now));
  });

  it("halves the calendar time when the pace doubles", () => {
    const now = new Date(2026, 7, 17);
    const p = progress({ completed: 55, total: 110 });

    const single = momentum(p, now, { perDay: 1, daysPerWeek: 5 });
    const double = momentum(p, now, { perDay: 2, daysPerWeek: 5 });

    expect(single.weeksLeft).toBe(11);
    expect(double.weeksLeft).toBe(6); // 27.5 sittings rounds to 28, over 5 a week
  });

  it("counts calendar time, not study time, when weekends are off", () => {
    const now = new Date(2026, 7, 17);
    const p = progress({ completed: 100, total: 110 });

    // Ten cases, two a sitting, five sittings — one working week, which is
    // seven calendar days away and must not be reported as five.
    const m = momentum(p, now, { perDay: 2, daysPerWeek: 5 });
    expect(daysBetween(now, m.finishDate!)).toBe(7);
  });

  it("charges a whole sitting for a partial one", () => {
    // 11 cases at 2 a day is 5.5 sittings; you cannot do half of one.
    const m = momentum(progress({ completed: 99, total: 110 }), new Date(), {
      perDay: 2,
      daysPerWeek: 6,
    });
    expect(m.weeksLeft).toBe(1); // 6 sittings, exactly one week of them
  });

  it("projects nothing rather than Infinity on a zero pace", () => {
    const m = momentum(progress({ completed: 0, total: 110 }), new Date(), {
      perDay: 0,
      daysPerWeek: 5,
    });
    expect(m.finishDate).toBeNull();
    expect(m.remaining).toBe(110);
  });
});

describe("paceForDeadline", () => {
  const now = new Date(2026, 7, 17);

  it("answers the forward projection's own question backwards", () => {
    // 55 left, ten weeks away, five days a week is fifty sittings — so 1.1 a
    // sitting. Arithmetic anybody can redo, which is the standard every number
    // in this file is held to.
    const needed = paceForDeadline(55, new Date(2026, 9, 26), 5, now)!;
    expect(needed.daysAvailable).toBe(70);
    expect(needed.perDay).toBeCloseTo(1.1, 10);
    expect(needed.perWeek).toBeCloseTo(5.5, 10);
  });

  // The two directions are one equation, so a pace taken from a deadline must
  // land back on that deadline when fed to the projection.
  it("round-trips through momentum", () => {
    const target = new Date(2026, 9, 26);
    const needed = paceForDeadline(55, target, 5, now)!;

    const m = momentum(progress({ completed: 55, total: 110 }), now, {
      perDay: needed.perDay,
      daysPerWeek: 5,
    });
    expect(m.finishDate!.toDateString()).toBe(target.toDateString());
  });

  it("says everything-today rather than infinity for a date gone by", () => {
    const needed = paceForDeadline(55, new Date(2026, 7, 10), 5, now)!;
    expect(needed.tooLate).toBe(true);
    expect(needed.perDay).toBe(55);
    expect(needed.daysAvailable).toBeLessThan(0);
  });

  it("treats today as too late — there is no time left to spread over", () => {
    expect(paceForDeadline(55, now, 5, now)!.tooLate).toBe(true);
  });

  it("has nothing to schedule once the plan is done", () => {
    expect(paceForDeadline(0, new Date(2026, 9, 26), 5, now)).toBeNull();
  });

  it("refuses a week with no study days in it", () => {
    expect(paceForDeadline(55, new Date(2026, 9, 26), 0, now)).toBeNull();
  });
});

describe("daysBetween", () => {
  it("ignores the time of day on either side", () => {
    const from = new Date(2026, 7, 17, 23, 55);
    const to = new Date(2026, 7, 18, 0, 5);
    expect(daysBetween(from, to)).toBe(1);
  });

  // A day containing a daylight-saving change is 23 or 25 hours long. Dividing
  // and flooring turns that into an off-by-one in every pace derived from it.
  it("counts a daylight-saving week as seven days", () => {
    const from = new Date(2026, 2, 25);
    const to = new Date(2026, 3, 1);
    expect(daysBetween(from, to)).toBe(7);
  });
});

describe("formatRate", () => {
  it("writes whole numbers plainly", () => {
    expect(formatRate(2)).toBe("2");
  });

  it("keeps one decimal where the fraction is the point", () => {
    expect(formatRate(1.1)).toBe("1.1");
    expect(formatRate(2.25)).toBe("2.3");
  });

  it("drops the decimal once the number is large enough not to need it", () => {
    expect(formatRate(12.4)).toBe("12");
  });
});

describe("loadPaceSettings", () => {
  beforeEach(() => localStorage.clear());

  it("falls back to the plan's cadence when nothing is saved", () => {
    expect(loadPaceSettings().pace).toEqual(DEFAULT_PACE);
  });

  it("round-trips what was saved", () => {
    savePaceSettings({
      pace: { perDay: 3, daysPerWeek: 6 },
      mode: "deadline",
      target: "2026-12-01",
    });
    expect(loadPaceSettings()).toEqual({
      pace: { perDay: 3, daysPerWeek: 6 },
      mode: "deadline",
      target: "2026-12-01",
    });
  });

  // A bad stored number does not fail loudly — it quietly produces a finish
  // date years out, which is the kind of wrong that gets believed.
  it("clamps a stored pace that is out of range", () => {
    localStorage.setItem(
      "excelplan:pace",
      JSON.stringify({ pace: { perDay: 0, daysPerWeek: 99 }, mode: "pace", target: "" }),
    );
    expect(loadPaceSettings().pace).toEqual(DEFAULT_PACE);
  });

  it("discards a target that is not a date", () => {
    localStorage.setItem(
      "excelplan:pace",
      JSON.stringify({ pace: DEFAULT_PACE, mode: "pace", target: "soon" }),
    );
    expect(loadPaceSettings().target).toBe("");
  });

  it("survives a corrupt stored value instead of failing the render", () => {
    localStorage.setItem("excelplan:pace", "not json");
    expect(() => loadPaceSettings()).not.toThrow();
    expect(loadPaceSettings().pace).toEqual(DEFAULT_PACE);
  });
});

describe("completePhaseKeys", () => {
  it("counts only phases that are fully done", () => {
    const p = progress({
      phases: [
        { key: "0", name: "", weekRange: "", order: 0, completed: 5, total: 5, weeks: [] },
        { key: "1", name: "", weekRange: "", order: 1, completed: 4, total: 15, weeks: [] },
      ],
    });
    expect(completePhaseKeys(p)).toEqual(["0"]);
  });

  // An empty phase is not an achievement. Without the total > 0 guard a phase
  // with no notes yet would satisfy completed >= total and celebrate itself.
  it("ignores a phase with no cases in it", () => {
    const p = progress({
      phases: [
        { key: "9", name: "", weekRange: "", order: 9, completed: 0, total: 0, weeks: [] },
      ],
    });
    expect(completePhaseKeys(p)).toEqual([]);
  });
});

describe("takeNewlyComplete", () => {
  beforeEach(() => localStorage.clear());

  const withPhase0Done = progress({
    phases: [
      { key: "0", name: "", weekRange: "", order: 0, completed: 5, total: 5, weeks: [] },
    ],
  });

  it("reports a phase the first time it is finished", () => {
    expect(takeNewlyComplete(withPhase0Done)).toEqual(["0"]);
  });

  // The banner must not reappear on every reload. A congratulation that
  // repeats stops reading as congratulation and starts reading as a bug.
  it("stays quiet on every later call", () => {
    takeNewlyComplete(withPhase0Done);
    expect(takeNewlyComplete(withPhase0Done)).toEqual([]);
  });

  it("still reports a second phase finished later", () => {
    takeNewlyComplete(withPhase0Done);

    const alsoPhase1 = progress({
      phases: [
        ...withPhase0Done.phases,
        { key: "1", name: "", weekRange: "", order: 1, completed: 15, total: 15, weeks: [] },
      ],
    });
    expect(takeNewlyComplete(alsoPhase1)).toEqual(["1"]);
  });

  it("survives a corrupt stored value instead of failing the render", () => {
    localStorage.setItem("excelplan:celebrated-phases", "not json");
    expect(() => takeNewlyComplete(withPhase0Done)).not.toThrow();
  });
});
