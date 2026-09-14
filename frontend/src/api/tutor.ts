/**
 * The case tutor.
 *
 * Every call goes through the backend, never to Gemini directly — the API key
 * lives on the server, and a browser calling the model would ship that key to
 * anyone who opened dev tools. The backend also knows the vault, which is where
 * the case context the tutor needs actually comes from.
 */

import { ApiError } from "./progress";

const BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8090";

/**
 * One piece of an answer. `type` decides which other fields are set, which is
 * what lets a table render as a table instead of as pipe characters in a
 * paragraph.
 */
export interface Block {
  type: "text" | "table" | "code" | "steps";
  text?: string;
  columns?: string[];
  rows?: string[][];
  language?: string;
  code?: string;
  items?: string[];
}

export interface TutorMessage {
  role: "user" | "model";
  /** What the student typed. Absent on a model turn. */
  text?: string;
  /** The answer. Absent on a student turn. */
  blocks?: Block[];
  at: string;
}

export interface TutorThread {
  messages: TutorMessage[] | null;
  /** False when no API key is set, so the tab explains setup instead of
   *  offering an input that cannot work. */
  configured: boolean;
  model?: string;
  /** True when this case has a worked example, so the tutor is reciting the
   *  intended answer rather than deriving one. */
  hasExample: boolean;
}

async function readOrThrow(res: Response): Promise<TutorThread> {
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as TutorThread;
}

const url = (slug: string) => `${BASE}/v1/notes/${encodeURIComponent(slug)}/tutor`;

export async function fetchThread(slug: string): Promise<TutorThread> {
  let res: Response;
  try {
    res = await fetch(url(slug));
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  return readOrThrow(res);
}

/** Ask one question. Answers with the whole thread, not just the reply, so the
 *  transcript on screen is the one the server stored. */
export async function ask(slug: string, question: string): Promise<TutorThread> {
  let res: Response;
  try {
    res = await fetch(url(slug), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ question }),
    });
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  return readOrThrow(res);
}

export async function clearThread(slug: string): Promise<TutorThread> {
  let res: Response;
  try {
    res = await fetch(url(slug), { method: "DELETE" });
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  return readOrThrow(res);
}
