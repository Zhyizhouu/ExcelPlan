import { useEffect, useState } from "react";
import { ApiError } from "../api/progress";
import { fetchJudge, type JudgeReport } from "../api/judge";
import { Button } from "./ui";

/**
 * Checks the saved workbook against the expected result.
 *
 * The saved time is shown with every verdict: the checker reads the file on
 * disk, and an answer typed but not saved otherwise looks like a wrong one.
 */
export function JudgePanel({ slug, workOnData = false }: { slug: string; workOnData?: boolean }) {
  const [report, setReport] = useState<JudgeReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setReport(null);
    setError(null);
  }, [slug]);

  async function onCheck() {
    setBusy(true);
    setError(null);
    try {
      setReport(await fetchJudge(slug));
    } catch (err) {
      setReport(null);
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const failed = report?.checks.filter((c) => c.status === "fail").length ?? 0;

  return (
    <section>
      <h2 className="mb-1 text-lg font-semibold text-ink">Check your answer</h2>
      <p className="mb-3 text-sm text-ink-muted">
        {workOnData
          ? "Do the work on the Data sheet, or on a copy of the data on the Answer sheet"
          : "Build your table on the Answer sheet with the same headers as the expected result"}
        , save in Excel (Ctrl+S), then check. The checker reads the saved file.
      </p>
      <Button onClick={() => void onCheck()} disabled={busy}>
        {busy ? "Checking…" : report ? "Check again" : "Check my answer"}
      </Button>

      {error && (
        <p role="status" className="mt-3 text-sm text-warning">
          {error}
        </p>
      )}

      {report && (
        <div className="mt-4 rounded-lg border border-line bg-surface p-4">
          <p
            role="status"
            className={`text-sm font-medium ${report.passed ? "text-accent" : "text-warning"}`}
          >
            {report.passed
              ? "Every check passes."
              : `${failed} of ${report.checks.length} checks fail.`}
          </p>
          <p className="mt-1 text-xs text-ink-muted">
            Checked the version saved {savedAt(report.savedAt)}.
          </p>

          <ul className="mt-3 space-y-3">
            {report.checks.map((c) => (
              <li key={c.id} className="flex gap-2.5">
                <span
                  aria-hidden
                  className={`mt-0.5 w-3 shrink-0 font-mono text-sm ${
                    c.status === "pass" ? "text-accent" : "text-warning"
                  }`}
                >
                  {c.status === "pass" ? "✓" : "✗"}
                </span>
                <div className="min-w-0">
                  <p className="text-sm text-ink">
                    <span className="sr-only">
                      {c.status === "pass" ? "Passed: " : "Failed: "}
                    </span>
                    {c.label}
                  </p>
                  {c.detail && (
                    <p className="mt-0.5 text-xs leading-relaxed text-ink-muted">{c.detail}</p>
                  )}
                  {c.items && c.items.length > 0 && (
                    <ul className="mt-1 list-disc space-y-0.5 pl-4 text-xs text-ink-muted">
                      {c.items.map((item, i) => (
                        <li key={i}>{item}</li>
                      ))}
                    </ul>
                  )}
                </div>
              </li>
            ))}
          </ul>

          {report.notChecked && report.notChecked.length > 0 && (
            <div className="mt-4 border-t border-line pt-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
                Not checked
              </p>
              <ul className="mt-1 list-disc space-y-0.5 pl-4 text-xs text-ink-muted">
                {report.notChecked.map((n, i) => (
                  <li key={i}>{n}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function savedAt(iso: string): string {
  const d = new Date(iso);
  if (d.toDateString() === new Date().toDateString()) {
    return `at ${d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  }
  return `on ${d.toLocaleString([], { dateStyle: "medium", timeStyle: "short" })}`;
}
