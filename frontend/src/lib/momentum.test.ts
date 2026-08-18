import { beforeEach, describe, expect, it } from "vitest";
import type { Progress } from "../api/progress";
import {
  CADENCE_PER_WEEK,
  completePhaseKeys,
  momentum,
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
