/**
 * The one call this app makes.
 *
 * The plan is 110 notes — small enough that fetching all of it at once is
 * cheaper than the round trips paginating it would cost, and it means every
 * screen renders from a single source rather than each assembling its own
 * partial view.
 */

const BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8090";

export interface Field {
  label: string;
  value: string;
}

export interface Note {
  Slug: string;
  Title: string;
  Phase: string;
  Week: number;
  Created: string;
  Complete: boolean;
  Path: string;
  /** The note's `# ...` line, often more specific than the title. */
  Heading: string;
  /** The `**Label:** value` lines that make up the case. May be absent on a
   *  note that does not follow the usual shape. */
  Fields: Field[] | null;
}

export interface WeekProgress {
  phase: string;
  week: number;
  completed: number;
  total: number;
  notes: Note[];
}

export interface PhaseProgress {
  key: string;
  name: string;
  weekRange: string;
  order: number;
  completed: number;
  total: number;
  weeks: WeekProgress[];
}

export interface Streak {
  current: number;
  longest: number;
  lastActive?: string;
}

export interface Progress {
  completed: number;
  total: number;
  nextUp?: Note;
  phases: PhaseProgress[];
  streak: Streak;
}

export class ApiError extends Error {
  /** True when the backend could not be reached at all, as opposed to
   *  reaching it and getting a refusal. The two need different advice: one is
   *  "start the server", the other is "something is wrong with the vault". */
  readonly offline: boolean;

  constructor(message: string, offline = false) {
    super(message);
    this.name = "ApiError";
    this.offline = offline;
  }
}

export async function fetchProgress(): Promise<Progress> {
  let res: Response;
  try {
    res = await fetch(`${BASE}/v1/progress`);
  } catch {
    throw new ApiError(
      "Can't reach the ExcelPlan server. Start it with `go run ./cmd/api` in the backend folder.",
      true,
    );
  }

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as Progress;
}

/**
 * Tick or untick a case, writing the checkbox in the note itself.
 *
 * Answers with the whole refreshed plan rather than an acknowledgement,
 * because one tick moves the week, the phase, the total, the streak and which
 * case is next — and re-deriving those on the client is how the screen and
 * the file start disagreeing.
 */
export async function setComplete(slug: string, complete: boolean): Promise<Progress> {
  let res: Response;
  try {
    res = await fetch(`${BASE}/v1/notes/${encodeURIComponent(slug)}/complete`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ complete }),
    });
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server — the tick wasn't saved.", true);
  }

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as Progress;
}

export interface SheetTableData {
  columns: string[];
  rows: Array<Record<string, unknown>>;
}

export interface Example {
  dataset: string;
  inputColumns: string[];
  inputNote: string;
  task: string;
  result: SheetTableData;
  resultNote: string;
  formula: string;
}

export interface NoteDetail {
  note: Note;
  /** Absent for cases with no worked example written yet — most of them. */
  example?: Example;
  dataset?: SheetTableData & { id: string; name: string; description: string };
}

export async function fetchNote(slug: string): Promise<NoteDetail> {
  let res: Response;
  try {
    res = await fetch(`${BASE}/v1/notes/${encodeURIComponent(slug)}`);
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as NoteDetail;
}

/** Direct URL for the .xlsx, used as an anchor href so the browser downloads
 *  it with its own filename rather than one invented in JavaScript. */
export const workbookUrl = (slug: string) =>
  `${BASE}/v1/notes/${encodeURIComponent(slug)}/workbook.xlsx`;

/** The input table as CSV text, for the clipboard. */
export async function fetchCsv(slug: string): Promise<string> {
  const res = await fetch(`${BASE}/v1/notes/${encodeURIComponent(slug)}/data.csv`);
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return res.text();
}

/** Where the workbook is, and what just happened to it. */
export interface SavedWorkbook {
  path: string;
  bytes: number;
  /** False when it was already in the vault and was left exactly as it is. */
  created: boolean;
  opened: boolean;
}

/**
 * Make sure the case's workbook is in its phase folder, optionally opening it.
 *
 * Writes it if it isn't there, and leaves it strictly alone if it is — that
 * file is where the answers get typed, so it is never written over. Asking a
 * second time is therefore not an error; it just finds the file already there.
 *
 * Opening is the server's job, not the browser's: the file is on the same
 * machine, and a page has no way to hand a local path to Excel.
 */
export async function saveWorkbook(
  slug: string,
  open: boolean,
): Promise<SavedWorkbook> {
  const url =
    `${BASE}/v1/notes/${encodeURIComponent(slug)}/workbook` + (open ? "?open=1" : "");
  let res: Response;
  try {
    res = await fetch(url, { method: "POST" });
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as SavedWorkbook;
}

/** Derived status for a week or phase, used for badges and bar colour. */
export type Status = "not_started" | "in_progress" | "complete";

export function statusOf(completed: number, total: number): Status {
  if (total > 0 && completed >= total) return "complete";
  return completed > 0 ? "in_progress" : "not_started";
}
