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

export interface Momentum {
  remaining: number;
  percent: number;
  /** Null once nothing remains — there is no date to project to. */
  finishDate: Date | null;
  /** Whole weeks left at the stated cadence, rounded up. */
  weeksLeft: number;
}

export function momentum(progress: Progress, now = new Date()): Momentum {
  const remaining = Math.max(0, progress.total - progress.completed);
  const percent = progress.total === 0 ? 0 : (progress.completed / progress.total) * 100;

  // An empty plan has nothing remaining, but it is not finished — reporting
  // 100% for "no cases at all" would show a full bar the moment the vault
  // could not be read, which is the worst possible time to look done.
  if (remaining === 0) {
    return { remaining: 0, percent, finishDate: null, weeksLeft: 0 };
  }

  const weeksLeft = Math.ceil(remaining / CADENCE_PER_WEEK);
  const finishDate = new Date(now);
  finishDate.setDate(finishDate.getDate() + weeksLeft * 7);

  return { remaining, percent, finishDate, weeksLeft };
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

export function formatDate(date: Date): string {
  return date.toLocaleDateString(undefined, {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
