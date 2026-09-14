import { ApiError } from "./progress";

const BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8090";

export interface JudgeCheck {
  id: string;
  label: string;
  status: "pass" | "fail";
  detail?: string;
  items?: string[];
}

export interface JudgeReport {
  passed: boolean;
  checks: JudgeCheck[];
  /** What the checker cannot see, so a pass is not read as covering it. */
  notChecked?: string[];
  path: string;
  savedAt: string;
}

/** Checks the case's workbook in the vault, as last saved. */
export async function fetchJudge(slug: string): Promise<JudgeReport> {
  let res: Response;
  try {
    res = await fetch(`${BASE}/v1/notes/${encodeURIComponent(slug)}/judge`);
  } catch {
    throw new ApiError("Can't reach the ExcelPlan server.", true);
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(body?.error ?? `The server answered ${res.status}.`);
  }
  return (await res.json()) as JudgeReport;
}
