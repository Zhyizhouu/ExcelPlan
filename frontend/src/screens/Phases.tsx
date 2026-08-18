import { useEffect, useState } from "react";
import type { PhaseProgress, Progress, WeekProgress } from "../api/progress";
import { statusOf } from "../api/progress";
import { Badge, Card, ProgressBar } from "../components/ui";
import { CaseCard } from "../components/CaseCard";

const OPEN_KEY = "excelplan:open-phases";

/**
 * Every phase, collapsible, with the cases inside.
 *
 * Phases start collapsed except the one being worked on. 110 cases open at
 * once is a wall rather than a plan, and the whole point of collapsing is
 * that the section you are on should be the only thing asking for attention.
 *
 * Which phases are open persists, because reopening the same one on every
 * visit is exactly the kind of small friction that stops a tool being used.
 */
export function Phases({
  progress,
  onToggle,
  onOpen,
  busySlug,
  slugError,
}: {
  progress: Progress;
  onToggle: (slug: string, complete: boolean) => void;
  onOpen: (slug: string) => void;
  busySlug: string | null;
  slugError: { slug: string; message: string } | null;
}) {
  const activeKey = progress.nextUp?.Phase ?? progress.phases[0]?.key ?? null;
  const [open, setOpen] = useState<string[]>([]);

  // Restore on mount, defaulting to the phase the next case lives in. Done in
  // an effect rather than a lazy initialiser so the default can depend on
  // progress, which is not available until the fetch lands.
  useEffect(() => {
    let stored: string[] | null = null;
    try {
      const raw = localStorage.getItem(OPEN_KEY);
      if (raw) stored = JSON.parse(raw) as string[];
    } catch {
      stored = null;
    }
    setOpen(stored ?? (activeKey ? [activeKey] : []));
    // Only on mount: re-running when activeKey changes would slam a phase
    // shut the moment its last case is ticked.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function toggleOpen(key: string) {
    setOpen((prev) => {
      const next = prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key];
      try {
        localStorage.setItem(OPEN_KEY, JSON.stringify(next));
      } catch {
        /* private mode — the panel just will not be remembered */
      }
      return next;
    });
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-3xl font-semibold text-ink">All phases</h1>
        <p className="mt-1 text-sm text-ink-muted">
          {progress.total} cases across {progress.phases.length} phases. Ticking a
          box here writes straight to the note in your vault.
        </p>
      </header>

      {progress.phases.map((phase) => (
        <PhaseSection
          key={phase.key}
          phase={phase}
          open={open.includes(phase.key)}
          isCurrent={phase.key === activeKey}
          onOpenToggle={() => toggleOpen(phase.key)}
          onToggle={onToggle}
          onOpen={onOpen}
          busySlug={busySlug}
          slugError={slugError}
        />
      ))}
    </div>
  );
}

function PhaseSection({
  phase,
  open,
  isCurrent,
  onOpenToggle,
  onToggle,
  onOpen,
  busySlug,
  slugError,
}: {
  phase: PhaseProgress;
  open: boolean;
  isCurrent: boolean;
  onOpenToggle: () => void;
  onToggle: (slug: string, complete: boolean) => void;
  onOpen: (slug: string) => void;
  busySlug: string | null;
  slugError: { slug: string; message: string } | null;
}) {
  const status = statusOf(phase.completed, phase.total);
  const panelId = `phase-${phase.key}`;

  return (
    <Card className={`p-0 ${isCurrent ? "border-accent/40" : ""}`}>
      {/*
        The whole header is the control, not a small chevron beside it — a
        collapse target you have to aim for is one you stop using.
      */}
      <button
        type="button"
        onClick={onOpenToggle}
        aria-expanded={open}
        aria-controls={panelId}
        className="flex w-full items-center gap-3 p-5 text-left transition-colors
          hover:bg-accent-soft/40 focus-visible:outline-none focus-visible:ring-2
          focus-visible:ring-accent"
      >
        <span
          aria-hidden
          className={`shrink-0 text-xs text-ink-muted transition-transform duration-200
            ${open ? "rotate-90" : ""}`}
        >
          ▶
        </span>

        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-baseline gap-x-2">
            <span className="font-serif text-base font-semibold text-ink">
              Phase {phase.key}
              {phase.name ? ` — ${phase.name}` : ""}
            </span>
            {isCurrent && (
              <span className="text-xs font-medium text-accent">current</span>
            )}
          </span>
          <span className="mt-0.5 block text-xs text-ink-muted">
            {phase.weekRange}
          </span>
        </span>

        <span className="hidden w-36 shrink-0 sm:block">
          <ProgressBar
            value={phase.completed}
            total={phase.total}
            label={`Phase ${phase.key}`}
          />
        </span>
        <span className="shrink-0 font-mono text-xs text-ink-muted">
          {phase.completed}/{phase.total}
        </span>
        <Badge status={status} />
      </button>

      {open && (
        <div id={panelId} className="ep-rise border-t border-line p-5 pt-4">
          <div className="space-y-5">
            {phase.weeks.map((week) => (
              <WeekBlock
                key={`${week.phase}-${week.week}`}
                week={week}
                onToggle={onToggle}
                onOpen={onOpen}
                busySlug={busySlug}
                slugError={slugError}
              />
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}

function WeekBlock({
  week,
  onToggle,
  onOpen,
  busySlug,
  slugError,
}: {
  week: WeekProgress;
  onToggle: (slug: string, complete: boolean) => void;
  onOpen: (slug: string) => void;
  busySlug: string | null;
  slugError: { slug: string; message: string } | null;
}) {
  return (
    <section>
      <div className="mb-1 flex items-baseline justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">Week {week.week}</h3>
        <span className="font-mono text-xs text-ink-muted">
          {week.completed}/{week.total}
        </span>
      </div>
      <ProgressBar
        value={week.completed}
        total={week.total}
        label={`Week ${week.week}`}
        className="mb-1"
      />
      <ul>
        {week.notes.map((note) => (
          <CaseCard
            key={note.Slug}
            note={note}
            onToggle={onToggle}
            onOpen={onOpen}
            busy={busySlug === note.Slug}
            error={slugError?.slug === note.Slug ? slugError.message : undefined}
          />
        ))}
      </ul>
    </section>
  );
}
