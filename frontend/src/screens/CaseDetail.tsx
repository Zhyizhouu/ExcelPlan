import { useEffect, useState } from "react";
import { ApiError, fetchNote, type NoteDetail } from "../api/progress";
import { Button, Card, Skeleton } from "../components/ui";
import { SheetTable } from "../components/SheetTable";
import { ExportBar } from "../components/ExportBar";
import { TutorPanel } from "../components/TutorPanel";
import { JudgePanel } from "../components/JudgePanel";

type Pane = "case" | "tutor";

/**
 * One case, in full: what it asks, the data it works against, and what the
 * answer should look like.
 *
 * A page rather than an expanding row. The content is a task plus two tables,
 * which is more than fits comfortably inside a list, and a case is something
 * you sit with for twenty minutes rather than glance at.
 */
export function CaseDetail({
  slug,
  complete: completeFromPlan,
  onBack,
  onToggle,
  busy,
  error: toggleError,
}: {
  slug: string;
  /** The tick as the shared plan has it — authoritative over the snapshot this
   *  page fetched, which never changes after it loads. */
  complete?: boolean;
  onBack: () => void;
  onToggle: (slug: string, complete: boolean) => void;
  busy: boolean;
  /** Set when a tick from this page failed to reach the note. */
  error?: string | null;
}) {
  const [detail, setDetail] = useState<NoteDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pane, setPane] = useState<Pane>("case");

  useEffect(() => {
    let cancelled = false;
    setDetail(null);
    setError(null);
    setPane("case"); // a new case opens on the brief, never mid-conversation

    fetchNote(slug)
      .then((d) => !cancelled && setDetail(d))
      .catch((e) =>
        !cancelled && setError(e instanceof ApiError ? e.message : String(e)),
      );

    return () => {
      cancelled = true;
    };
  }, [slug]);

  if (error) {
    return (
      <div className="space-y-4">
        <BackLink onBack={onBack} />
        <Card className="border-warning/40">
          <p className="text-sm text-ink">{error}</p>
        </Card>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="space-y-4">
        <BackLink onBack={onBack} />
        <Skeleton className="h-8 w-80" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const { note } = detail;
  const title = (note.Heading || note.Title || note.Slug).replace(
    new RegExp(`^${note.Slug}\\s*[-–—]\\s*`),
    "",
  );
  const complete = completeFromPlan ?? note.Complete;

  return (
    <article className="space-y-7">
      <BackLink onBack={onBack} />

      <header>
        <p className="font-mono text-xs text-ink-muted">
          {note.Slug} · Phase {note.Phase} · Week {note.Week}
        </p>
        <h1 className="mt-1 font-serif text-3xl font-semibold leading-tight text-ink">
          {title}
        </h1>

        <label
          className="mt-4 inline-flex cursor-pointer items-center gap-2.5 rounded-lg
            border border-line bg-surface px-3 py-2 transition-colors hover:border-accent"
        >
          <input
            type="checkbox"
            checked={complete}
            disabled={busy}
            onChange={(e) => onToggle(note.Slug, e.target.checked)}
            className="h-4 w-4 cursor-pointer accent-[rgb(var(--color-accent))]
              disabled:cursor-wait disabled:opacity-50"
          />
          <span className="text-sm text-ink">
            {complete ? "Completed" : "Mark complete"}
          </span>
        </label>

        {/* A tick that did not reach the note has rolled back on screen, which
            on its own looks like a misclick. Said here, next to the box. */}
        {toggleError && (
          <p role="status" className="mt-2 text-xs text-warning">
            {toggleError}
          </p>
        )}
      </header>

      {/*
        Two panes rather than the tutor stacked under the brief. The brief is
        read once and the conversation runs long, so one page would mean
        scrolling past the case every time you asked something — and the tutor
        is wanted precisely when you are mid-attempt.
      */}
      <div className="flex gap-1 border-b border-line" role="tablist">
        {(["case", "tutor"] as const).map((p) => (
          <button
            key={p}
            type="button"
            role="tab"
            aria-selected={pane === p}
            onClick={() => setPane(p)}
            className={`-mb-px border-b-2 px-4 py-2 text-sm transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent ${
                pane === p
                  ? "border-accent font-medium text-ink"
                  : "border-transparent text-ink-muted hover:text-ink"
              }`}
          >
            {p === "case" ? "The case" : "Tutor"}
          </button>
        ))}
      </div>

      {pane === "tutor" ? (
        <TutorPanel slug={slug} />
      ) : (
        <CaseBody slug={slug} detail={detail} />
      )}
    </article>
  );
}

/**
 * The brief itself: the note's own fields, then the worked example if the case
 * has one.
 *
 * Its own component so the tab switch above is one line rather than a wrapper
 * around eighty, and so the two panes read as the alternatives they are.
 */
function CaseBody({ slug, detail }: { slug: string; detail: NoteDetail }) {
  const { note, example, dataset } = detail;

  return (
    <div className="space-y-7">
      {/* The case as written in the vault. */}
      {note.Fields && note.Fields.length > 0 && (
        <Card>
          <dl className="space-y-3">
            {note.Fields.map((field) => (
              <div key={field.label} className="sm:flex sm:gap-4">
                <dt className="w-28 shrink-0 text-xs font-semibold uppercase tracking-wide text-ink-muted">
                  {field.label}
                </dt>
                <dd className="mt-0.5 text-sm leading-relaxed text-ink sm:mt-0">
                  {field.value}
                </dd>
              </div>
            ))}
          </dl>
        </Card>
      )}

      {example ? (
        <>
          <section>
            <h2 className="mb-1 text-lg font-semibold text-ink">The task</h2>
            <p className="text-sm leading-relaxed text-ink-muted">{example.task}</p>

            {/*
              Named up front, not buried in the task text. A case whose stated
              objective the answer never actually needs is one you can finish
              without learning the thing it was written for — and the sheet
              looks the same either way, so nothing tells you.
            */}
            {example.requires && example.requires.length > 0 && (
              <p className="mt-3 flex flex-wrap items-center gap-2 text-xs text-ink-muted">
                <span className="font-semibold uppercase tracking-wide">
                  Must use
                </span>
                {example.requires.map((r) => (
                  <span
                    key={r}
                    className="rounded-full border border-accent/40 bg-accent-soft px-2.5
                      py-0.5 font-mono text-accent"
                  >
                    {r}
                  </span>
                ))}
              </p>
            )}

            {example.controls && example.controls.length > 0 && (
              <div className="mt-3 rounded-lg border border-line bg-surface p-4">
                {example.controls.map((c) => (
                  <div key={c.cell}>
                    <p className="text-sm text-ink">
                      <span className="font-mono text-xs text-ink-muted">{c.cell}</span>{" "}
                      <span className="font-medium">{c.label}</span> ={" "}
                      <span className="font-mono">{c.value}</span>
                    </p>
                    <p className="mt-1 text-xs leading-relaxed text-ink-muted">{c.note}</p>
                  </div>
                ))}
              </div>
            )}
          </section>

          {dataset && (
            <section>
              <h2 className="mb-1 text-lg font-semibold text-ink">Input</h2>
              <p className="mb-3 text-sm text-ink-muted">{example.inputNote}</p>
              <SheetTable
                data={dataset}
                maxRows={12}
                caption={`${dataset.name} — the columns this case uses`}
                highlight={example.inputColumns}
              />
              <div className="mt-4">
                <ExportBar
                  slug={slug}
                  checkerNeedsAnswerSheet={
                    !example.practice && !example.reference && example.workOn !== "data"
                  }
                />
              </div>
            </section>
          )}

          <section>
            <h2 className="mb-1 text-lg font-semibold text-ink">Expected result</h2>
            <p className="mb-3 text-sm text-ink-muted">{example.resultNote}</p>
            <SheetTable data={example.result} caption="What your sheet should show" />
          </section>

          {/* Practice and reference examples are not checked. */}
          {!example.practice && !example.reference && (
            <JudgePanel slug={slug} workOnData={example.workOn === "data"} />
          )}

          {example.formula && (
            <section>
              <h2 className="mb-2 text-lg font-semibold text-ink">The formula</h2>
              <pre
                className="overflow-x-auto rounded-lg border border-line bg-surface p-4
                  font-mono text-xs leading-relaxed text-ink"
              >
                {example.formula}
              </pre>
            </section>
          )}
        </>
      ) : (
        /*
         * Said plainly rather than hidden. A page that just stops after the
         * note fields reads like something failed to load; naming the gap
         * makes it obviously a gap in the content, not in the app.
         *
         * The count comes from the server rather than being written here. This
         * card used to name the phases it believed were covered, and that claim
         * went stale the day the weekly-build cases landed inside two of them —
         * a sentence asserting a fact that nothing was checking.
         */
        <Card className="border-dashed">
          <p className="text-sm text-ink-muted">
            No worked example for this case yet — the note above is the whole
            brief.{" "}
            {detail.exampleCount
              ? `${detail.exampleCount} cases have one so far; writing them is hand work, and a wrong one would be worse than none.`
              : "Writing them is hand work, and a wrong one would be worse than none."}{" "}
            The Tutor tab can still talk you through this one.
          </p>
        </Card>
      )}
    </div>
  );
}

function BackLink({ onBack }: { onBack: () => void }) {
  return (
    <Button variant="quiet" onClick={onBack} className="-ml-4 px-4">
      ← All phases
    </Button>
  );
}
