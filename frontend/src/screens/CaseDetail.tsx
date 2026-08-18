import { useEffect, useState } from "react";
import { ApiError, fetchNote, type NoteDetail } from "../api/progress";
import { Button, Card, Skeleton } from "../components/ui";
import { SheetTable } from "../components/SheetTable";
import { ExportBar } from "../components/ExportBar";

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

  useEffect(() => {
    let cancelled = false;
    setDetail(null);
    setError(null);

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

  const { note, example, dataset } = detail;
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
                <ExportBar slug={slug} />
              </div>
            </section>
          )}

          <section>
            <h2 className="mb-1 text-lg font-semibold text-ink">Expected result</h2>
            <p className="mb-3 text-sm text-ink-muted">{example.resultNote}</p>
            <SheetTable data={example.result} caption="What your sheet should show" />
          </section>

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
         */
        <Card className="border-dashed">
          <p className="text-sm text-ink-muted">
            No worked example for this case yet — the note above is the whole
            brief. Worked examples currently cover Phase 0 and Week 2.
          </p>
        </Card>
      )}
    </article>
  );
}

function BackLink({ onBack }: { onBack: () => void }) {
  return (
    <Button variant="quiet" onClick={onBack} className="-ml-4 px-4">
      ← All phases
    </Button>
  );
}
