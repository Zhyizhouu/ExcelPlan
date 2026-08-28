/**
 * The motivating numbers, all derived from real data.
 *
 * Every figure here is arithmetic on things the vault actually says — days
 * remaining, the stated cadence, dates the log recorded. Nothing is invented
 * to make a number look better, because a projection you stop believing is
 * worse than no projection: it teaches you to ignore the panel it lives in.
 */

import type { Progress } from "../api/progress";

/** The plan's own stated cadence: 5 cases a week, from the roadmap note. */
export const CADENCE_PER_WEEK = 5;

/**
 * How fast you actually work.
 *
 * Two numbers rather than one, because "a case a day" is ambiguous in exactly
 * the way that matters: five a week and seven a week are six calendar weeks
 * apart over a 110-case plan. Splitting the rate into cases-per-session and
 * sessions-per-week makes the assumption something you set and can see, rather
 * than something the arithmetic decided on your behalf.
 */
export interface Pace {
  /** Cases in one sitting, on a day you study. */
  perDay: number;
  /** Days a week you sit down at all. */
  daysPerWeek: number;
}

/**
 * The plan's own cadence, written as a pace: one case a day, five days a week.
 *
 * Chosen so the default projection is arithmetically identical to the fixed
 * one this replaced. Someone who never opens the pace card sees exactly the
 * date they saw before — the feature is opt-in, not a silent recalculation.
 */
export const DEFAULT_PACE: Pace = { perDay: 1, daysPerWeek: CADENCE_PER_WEEK };

/** The pace as a weekly rate, which is the only form the two modes share. */
export const casesPerWeek = (pace: Pace): number => pace.perDay * pace.daysPerWeek;

/** A pace that would divide by zero, or run backwards, projects nothing. */
export const isUsablePace = (pace: Pace): boolean =>
  Number.isFinite(pace.perDay) &&
  Number.isFinite(pace.daysPerWeek) &&
  pace.perDay > 0 &&
  pace.daysPerWeek > 0;

export interface Momentum {
  remaining: number;
  percent: number;
  /** Null once nothing remains — there is no date to project to. */
  finishDate: Date | null;
  /** Whole weeks left at the given pace, rounded up. */
  weeksLeft: number;
}

export function momentum(
  progress: Progress,
  now = new Date(),
  pace: Pace = DEFAULT_PACE,
): Momentum {
  const remaining = Math.max(0, progress.total - progress.completed);
  const percent = progress.total === 0 ? 0 : (progress.completed / progress.total) * 100;

  // An empty plan has nothing remaining, but it is not finished — reporting
  // 100% for "no cases at all" would show a full bar the moment the vault
  // could not be read, which is the worst possible time to look done.
  if (remaining === 0 || !isUsablePace(pace)) {
    return { remaining, percent, finishDate: null, weeksLeft: 0 };
  }

  // Sessions first, then calendar time. Rounding the sittings up before
  // spreading them over weeks is what makes the number honest: you cannot do
  // four fifths of a study session, so a remainder costs a whole one.
  const sessions = Math.ceil(remaining / pace.perDay);
  const weeks = sessions / pace.daysPerWeek;

  const finishDate = new Date(now);
  finishDate.setDate(finishDate.getDate() + Math.ceil(weeks * 7));

  return { remaining, percent, finishDate, weeksLeft: Math.ceil(weeks) };
}

const MS_PER_DAY = 24 * 60 * 60 * 1000;

/** Midnight local, so a comparison is between dates rather than moments. */
const startOfDay = (d: Date): Date =>
  new Date(d.getFullYear(), d.getMonth(), d.getDate());

/** Whole days from one date to another, ignoring the time of day. */
export function daysBetween(from: Date, to: Date): number {
  // Rounded, not floored: a day containing a daylight-saving change is 23 or
  // 25 hours long, and flooring turns that into an off-by-one in the pace.
  return Math.round(
    (startOfDay(to).getTime() - startOfDay(from).getTime()) / MS_PER_DAY,
  );
}

/** What finishing by a chosen date would actually take. */
export interface RequiredPace {
  /** Cases per study day needed to land on the target. */
  perDay: number;
  /** The same rate weekly, which is easier to sanity-check against the plan. */
  perWeek: number;
  /** Calendar days from today to the target. Zero or negative if it has gone. */
  daysAvailable: number;
  /** The target is today or already past, so no pace reaches it. */
  tooLate: boolean;
}

/**
 * The pace that hits a deadline — the question the other way round.
 *
 * Deliberately not rounded to something achievable-looking. If the honest
 * answer is 6.2 cases a study day, that is the number worth seeing, because
 * the useful outcome of asking is often finding out the date has to move.
 *
 * Null when there is nothing left to schedule, or the days-per-week is
 * unusable — both cases where any number returned would be made up.
 */
export function paceForDeadline(
  remaining: number,
  target: Date,
  daysPerWeek: number,
  now = new Date(),
): RequiredPace | null {
  if (remaining <= 0 || !Number.isFinite(daysPerWeek) || daysPerWeek <= 0) {
    return null;
  }

  const daysAvailable = daysBetween(now, target);
  if (daysAvailable <= 0) {
    // Everything, today. Said as a real number rather than Infinity, because
    // "55 a day" is a sentence and "∞" is a shrug.
    return { perDay: remaining, perWeek: remaining, daysAvailable, tooLate: true };
  }

  const sessions = (daysAvailable / 7) * daysPerWeek;
  const perDay = remaining / sessions;
  return { perDay, perWeek: perDay * daysPerWeek, daysAvailable, tooLate: false };
}

/**
 * A rate as text: whole numbers plainly, everything else to one decimal.
 *
 * "2 a day" and "2.3 a day" both read as answers; "2.0 a day" reads as a
 * spreadsheet leaking through.
 */
export function formatRate(n: number): string {
  if (!Number.isFinite(n)) return "—";
  if (Number.isInteger(n)) return String(n);
  return n >= 10 ? String(Math.round(n)) : n.toFixed(1);
}

/**
 * Phase keys that are fully complete.
 *
 * Used to detect a *newly* finished phase by comparing against what was
 * complete last time the dashboard rendered. A phase is the right size for a
 * celebration: a checkbox is too small to be worth interrupting for, and the
 * whole 110-case plan is too far away to feel reachable.
 */
export function completePhaseKeys(progress: Progress): string[] {
  return progress.phases
    .filter((p) => p.total > 0 && p.completed >= p.total)
    .map((p) => p.key);
}

const SEEN_KEY = "excelplan:celebrated-phases";

/**
 * Returns phase keys finished since the last call, and records them so the
 * same phase never celebrates twice.
 *
 * Persisted rather than held in memory: a refresh is not an achievement, and
 * a banner that reappears on every reload stops reading as congratulation and
 * starts reading as a bug.
 */
export function takeNewlyComplete(progress: Progress): string[] {
  const complete = completePhaseKeys(progress);

  let seen: string[] = [];
  try {
    seen = JSON.parse(localStorage.getItem(SEEN_KEY) ?? "[]") as string[];
  } catch {
    seen = []; // corrupt value is not worth failing a render over
  }

  const fresh = complete.filter((key) => !seen.includes(key));
  if (fresh.length > 0) {
    try {
      localStorage.setItem(SEEN_KEY, JSON.stringify(complete));
    } catch {
      // Private browsing or a full quota. Losing this only means the banner
      // may show again — not worth interrupting the page for.
    }
  }
  return fresh;
}

/** Which question the pace card is currently answering. */
export type PaceMode = "pace" | "deadline";

export interface PaceSettings {
  pace: Pace;
  mode: PaceMode;
  /** The deadline as `yyyy-mm-dd`, or "" when never set. */
  target: string;
}

export const DEFAULT_SETTINGS: PaceSettings = {
  pace: DEFAULT_PACE,
  mode: "pace",
  target: "",
};

const PACE_KEY = "excelplan:pace";
const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

const clampInt = (value: unknown, min: number, max: number, fallback: number): number => {
  const n = Math.round(Number(value));
  return Number.isFinite(n) && n >= min && n <= max ? n : fallback;
};

/**
 * The saved pace, or the plan's cadence if there isn't one.
 *
 * Every field is re-validated rather than trusted. This value is read from
 * storage a user can edit and a previous version of this app may have written,
 * and a bad number here does not fail loudly — it quietly produces a finish
 * date years out, which is the kind of wrong that gets believed.
 */
export function loadPaceSettings(): PaceSettings {
  let raw: unknown;
  try {
    raw = JSON.parse(localStorage.getItem(PACE_KEY) ?? "null");
  } catch {
    return DEFAULT_SETTINGS; // corrupt value is not worth failing a render over
  }
  if (!raw || typeof raw !== "object") return DEFAULT_SETTINGS;

  const saved = raw as Partial<PaceSettings> & { pace?: Partial<Pace> };
  const target = typeof saved.target === "string" && ISO_DATE.test(saved.target)
    ? saved.target
    : "";

  return {
    pace: {
      perDay: clampInt(saved.pace?.perDay, 1, 50, DEFAULT_PACE.perDay),
      daysPerWeek: clampInt(saved.pace?.daysPerWeek, 1, 7, DEFAULT_PACE.daysPerWeek),
    },
    mode: saved.mode === "deadline" ? "deadline" : "pace",
    target,
  };
}

export function savePaceSettings(settings: PaceSettings): void {
  try {
    localStorage.setItem(PACE_KEY, JSON.stringify(settings));
  } catch {
    // Private browsing or a full quota. The pace still works for this
    // session; it just will not be remembered.
  }
}

export function formatDate(date: Date): string {
  return date.toLocaleDateString(undefined, {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
