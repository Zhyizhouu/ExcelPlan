import { useEffect, useRef, useState } from "react";
import { ApiError } from "../api/progress";
import { ask, clearThread, fetchThread, type Block, type TutorThread } from "../api/tutor";
import { Button, Card } from "./ui";

/**
 * The tutor for one case.
 *
 * It already knows the case, the data and — where one exists — the intended
 * answer, so the opening move is a question, not an explanation of your own
 * homework. That is the whole design: the twenty seconds of context-setting
 * every general chatbot demands is the reason people stop using one mid-task.
 *
 * Answers arrive as typed blocks rather than markdown. The model is given a
 * schema, so a comparison comes back as a table with columns and rows, and this
 * renders it with components that match the rest of the app — no markdown
 * parser, no sanitiser, and no chance of layout the page was not designed for.
 */
export function TutorPanel({ slug }: { slug: string }) {
  const [thread, setThread] = useState<TutorThread | null>(null);
  const [question, setQuestion] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    setThread(null);
    setError(null);
    fetchThread(slug)
      .then((t) => !cancelled && setThread(t))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : String(e)));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  // Follow the conversation down as it grows. Only once an answer lands, not on
  // every keystroke, so typing a long question does not yank the page around.
  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [thread?.messages?.length]);

  async function send(text: string) {
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setError(null);
    setQuestion("");
    try {
      setThread(await ask(slug, trimmed));
    } catch (err) {
      // Put the question back in the box. Losing what you typed because the
      // network blinked is the rudest thing a chat box can do.
      setQuestion(trimmed);
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  if (error && !thread) {
    return (
      <Card className="border-warning/40">
        <p className="text-sm text-ink">{error}</p>
      </Card>
    );
  }

  if (!thread) {
    return <p className="text-sm text-ink-muted">Loading the conversation…</p>;
  }

  if (!thread.configured) {
    return <NotConfigured />;
  }

  const messages = thread.messages ?? [];

  return (
    <div className="space-y-4">
      {messages.length === 0 ? (
        <Opener onPick={(q) => void send(q)} hasExample={thread.hasExample} />
      ) : (
        <div className="space-y-5">
          {messages.map((m, i) =>
            m.role === "user" ? (
              <div key={i} className="flex justify-end">
                <p
                  className="max-w-[85%] whitespace-pre-wrap rounded-lg rounded-br-sm
                    bg-accent-soft px-3.5 py-2.5 text-sm text-ink"
                >
                  {m.text}
                </p>
              </div>
            ) : (
              <div key={i} className="space-y-3">
                {(m.blocks ?? []).map((b, j) => (
                  <BlockView key={j} block={b} />
                ))}
              </div>
            ),
          )}
        </div>
      )}

      {busy && (
        <p className="text-sm text-ink-muted" role="status">
          Thinking…
        </p>
      )}
      {error && (
        <p className="text-sm text-warning" role="status">
          {error}
        </p>
      )}
      <div ref={endRef} />

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void send(question);
        }}
        className="space-y-2"
      >
        <textarea
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={(e) => {
            // Enter sends, Shift+Enter breaks the line. Pasting a macro needs
            // real newlines, and reaching for a button to send does not.
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              void send(question);
            }
          }}
          rows={3}
          disabled={busy}
          placeholder="Ask about this case, or paste your formula or macro…"
          className="w-full resize-y rounded-lg border border-line bg-surface px-3 py-2
            text-sm text-ink placeholder:text-ink-muted transition-colors
            focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-60"
        />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-ink-muted">
            {thread.hasExample
              ? "This case has a worked example, so the tutor knows the intended answer."
              : "No worked example for this case — the tutor will give one good approach, not the intended one."}
          </p>
          <div className="flex gap-2">
            {messages.length > 0 && (
              <Button
                type="button"
                variant="quiet"
                disabled={busy}
                onClick={() => {
                  void clearThread(slug).then(setThread).catch(() => {});
                }}
              >
                Clear
              </Button>
            )}
            <Button type="submit" disabled={busy || !question.trim()}>
              {busy ? "Asking…" : "Ask"}
            </Button>
          </div>
        </div>
      </form>
    </div>
  );
}

/** Said plainly, with the exact thing to do. */
function NotConfigured() {
  return (
    <Card className="border-dashed">
      <h3 className="text-sm font-semibold text-ink">The tutor needs an API key</h3>
      <p className="mt-2 text-sm text-ink-muted">
        Put a Google AI Studio key in{" "}
        <span className="font-mono text-xs">backend/.env</span> as{" "}
        <span className="font-mono text-xs">GEMINI_API_KEY</span>, then restart the
        backend — it reads the file once at startup and will not pick up a change
        while running.
      </p>
      <p className="mt-2 text-xs text-ink-muted">
        Everything else in ExcelPlan works without one.
      </p>
    </Card>
  );
}

/**
 * Openers for an empty thread.
 *
 * A blank chat box is its own small barrier — the same "what do I do now"
 * that the dashboard's Next up card exists to remove. These are the three
 * things someone actually wants at the start of a case.
 */
function Opener({
  onPick,
  hasExample,
}: {
  onPick: (question: string) => void;
  hasExample: boolean;
}) {
  const starters = [
    "What is this case actually asking me to do?",
    hasExample
      ? "Give me a hint to get started — don't show me the answer yet."
      : "How should I approach this? Just point me in the right direction.",
    "Which functions should I be reaching for here, and why those?",
  ];

  return (
    <div>
      <p className="text-sm text-ink-muted">
        The tutor already knows this case, its data, and what it asks for. Ask
        anything — or start with one of these.
      </p>
      <div className="mt-3 flex flex-col gap-2">
        {starters.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => onPick(s)}
            className="rounded-lg border border-line bg-surface px-3.5 py-2.5 text-left
              text-sm text-ink transition-colors hover:border-accent
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          >
            {s}
          </button>
        ))}
      </div>
    </div>
  );
}

/**
 * One block, or nothing at all if it is empty.
 *
 * Every branch checks for content first. The model does occasionally emit a
 * block with the right type and no payload — an empty `code` block turned up on
 * the first live run — and each of these renders as visible furniture when
 * empty: a bordered box with nothing in it, or a stray bullet. The reader would
 * read that as something having failed to load. Dropping it is both the correct
 * rendering and the honest one, since a block with no content says nothing.
 */
function BlockView({ block }: { block: Block }) {
  switch (block.type) {
    case "table":
      return <ConceptTable columns={block.columns ?? []} rows={block.rows ?? []} />;

    case "code":
      if (!block.code?.trim()) return null;
      return (
        <pre
          className="overflow-x-auto rounded-lg border border-line bg-surface p-3.5
            font-mono text-xs leading-relaxed text-ink"
        >
          {block.code}
        </pre>
      );

    case "steps": {
      const items = (block.items ?? []).filter((i) => i.trim());
      if (items.length === 0) return null;
      return (
        <ol className="ml-5 list-decimal space-y-1.5 text-sm leading-relaxed text-ink">
          {items.map((item, i) => (
            <li key={i}>{item}</li>
          ))}
        </ol>
      );
    }

    default:
      if (!block.text?.trim()) return null;
      return (
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-ink">
          {block.text}
        </p>
      );
  }
}

/**
 * A plain table, deliberately not SheetTable.
 *
 * SheetTable draws column letters and row numbers because the cases talk in
 * `=LEFT(A2,4)`. A table comparing Sub against Function is not a spreadsheet,
 * and labelling its first column "A" would invite exactly the wrong reading.
 */
function ConceptTable({ columns, rows }: { columns: string[]; rows: string[][] }) {
  if (columns.length === 0 || rows.length === 0) return null;
  return (
    <div className="overflow-x-auto rounded-lg border border-line">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="bg-surface">
            {columns.map((c) => (
              <th
                key={c}
                scope="col"
                className="border-b border-line px-3 py-2 text-left text-xs font-semibold
                  uppercase tracking-wide text-ink-muted"
              >
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-line last:border-b-0">
              {columns.map((_, j) => (
                <td key={j} className="px-3 py-2 align-top leading-relaxed text-ink">
                  {row[j] ?? ""}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
