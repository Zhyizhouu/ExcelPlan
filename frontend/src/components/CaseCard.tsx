import type { Note } from "../api/progress";

/**
 * One case in a week's list: a checkbox, a title, and a click target.
 *
 * Layout note, because the first attempt got it wrong. The checkbox and the
 * title sit in a grid with a fixed first column rather than a flex row with a
 * nudged margin — with flex, a title that wraps to two lines pulls the
 * checkbox out of line with every other row, and the column of boxes stops
 * reading as a column. A grid keeps the boxes on one axis no matter how long
 * the titles get.
 *
 * The whole row opens the case; the checkbox alone ticks it. Those are
 * separate elements rather than one control doing both, so a click never has
 * to be guessed at.
 */
export function CaseCard({
  note,
  onToggle,
  onOpen,
  busy,
  error,
}: {
  note: Note;
  onToggle: (slug: string, complete: boolean) => void;
  onOpen: (slug: string) => void;
  busy: boolean;
  error?: string;
}) {
  // The frontmatter title already begins with the slug ("W1D1 - Keyboard
  // only"), so printing both stutters. Prefer the heading, which is sometimes
  // more specific, then strip the leading slug from whichever wins.
  const title = (note.Heading || note.Title || note.Slug).replace(
    new RegExp(`^${note.Slug}\\s*[-–—]\\s*`),
    "",
  );

  return (
    <li className="border-b border-line last:border-0">
      <div className="grid grid-cols-[1.25rem_1fr] items-start gap-x-3 gap-y-1 py-2">
        {/*
          Nudged down by 2px to sit on the title's first-line baseline rather
          than its box top — without it a 16px box against 20px line-height
          reads as floating high.
        */}
        <input
          type="checkbox"
          checked={note.Complete}
          disabled={busy}
          onChange={(e) => onToggle(note.Slug, e.target.checked)}
          onClick={(e) => e.stopPropagation()}
          aria-label={`Mark ${title} ${note.Complete ? "incomplete" : "complete"}`}
          className="mt-[3px] h-4 w-4 cursor-pointer accent-[rgb(var(--color-accent))]
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent
            disabled:cursor-wait disabled:opacity-50"
        />

        <button
          type="button"
          onClick={() => onOpen(note.Slug)}
          className="group min-w-0 text-left focus-visible:outline-none
            focus-visible:ring-2 focus-visible:ring-accent"
        >
          <span
            className={`block font-serif text-[0.9375rem] leading-5 transition-colors
              group-hover:text-accent ${
                note.Complete ? "text-ink-muted line-through" : "text-ink"
              }`}
          >
            {title}
          </span>
          <span className="mt-0.5 block font-mono text-xs text-ink-muted">
            {note.Slug}
          </span>
        </button>

        {error && (
          <p className="col-start-2 text-xs text-warning" role="alert">
            {error}
          </p>
        )}
      </div>
    </li>
  );
}
