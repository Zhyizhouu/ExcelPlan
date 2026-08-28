import { useId, type ReactNode } from "react";
import { Card } from "./ui";
import {
  casesPerWeek,
  formatDate,
  formatRate,
  isUsablePace,
  momentum,
  paceForDeadline,
  type PaceSettings,
} from "../lib/momentum";
import type { Progress } from "../api/progress";

/**
 * The same arithmetic, asked from both ends.
 *
 * "At this pace, when do I finish?" and "to finish by then, what pace do I
 * need?" are one equation with a different unknown, so they are one card with
 * a switch rather than two panels competing for the same corner of the screen.
 *
 * The deadline mode is the one that earns its place. A projected date you do
 * not like tells you nothing about what to change; being told the date costs
 * three cases a day instead of one is a decision you can act on — often by
 * moving the date, which is a legitimate outcome of having asked.
 */
export function PaceCard({
  progress,
  settings,
  onChange,
}: {
  progress: Progress;
  settings: PaceSettings;
  onChange: (next: PaceSettings) => void;
}) {
  const ids = useId();
  const { pace, mode } = settings;

  const remaining = Math.max(0, progress.total - progress.completed);
  const set = (patch: Partial<PaceSettings>) => onChange({ ...settings, ...patch });

  if (remaining === 0) {
    return (
      <Card>
        <CardLabel />
        <p className="mt-2 text-sm text-ink-muted">
          Nothing left to pace — every case is done.
        </p>
      </Card>
    );
  }

  const daysOff = 7 - pace.daysPerWeek;

  return (
    <Card>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <CardLabel />
        <div
          className="flex gap-1 rounded-lg border border-line p-0.5"
          role="group"
          aria-label="What to work out"
        >
          <ModeButton active={mode === "pace"} onClick={() => set({ mode: "pace" })}>
            From a pace
          </ModeButton>
          <ModeButton
            active={mode === "deadline"}
            onClick={() => set({ mode: "deadline" })}
          >
            From a date
          </ModeButton>
        </div>
      </div>

      {mode === "pace" ? (
        <FromPace ids={ids} settings={settings} progress={progress} onChange={set} />
      ) : (
        <FromDate ids={ids} settings={settings} remaining={remaining} onChange={set} />
      )}

      {/* The days-a-week input is shared by both modes, so the sentence that
          explains what it means belongs to the card, not to either half. */}
      <p className="mt-4 border-t border-line pt-3 text-xs text-ink-muted">
        A study day is a day you actually sit down —{" "}
        {daysOff === 0
          ? "seven a week means no days off"
          : `${pace.daysPerWeek} a week leaves ${daysOff} ${daysOff === 1 ? "day" : "days"} off`}
        . Either way the projection counts real calendar time.
      </p>
    </Card>
  );
}

/** Pace in, finish date out. */
function FromPace({
  ids,
  settings,
  progress,
  onChange,
}: {
  ids: string;
  settings: PaceSettings;
  progress: Progress;
  onChange: (patch: Partial<PaceSettings>) => void;
}) {
  const { pace } = settings;
  const m = momentum(progress, new Date(), pace);
  const usable = isUsablePace(pace);

  return (
    <>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <NumberField
          id={`${ids}-per-day`}
          label="Cases a study day"
          value={pace.perDay}
          min={1}
          max={50}
          onChange={(perDay) => onChange({ pace: { ...pace, perDay } })}
        />
        <NumberField
          id={`${ids}-days`}
          label="Study days a week"
          value={pace.daysPerWeek}
          min={1}
          max={7}
          onChange={(daysPerWeek) => onChange({ pace: { ...pace, daysPerWeek } })}
        />
      </div>

      <Outcome
        lead="Finishing around"
        headline={usable && m.finishDate ? formatDate(m.finishDate) : "—"}
        detail={
          usable
            ? `${m.remaining} left · ${casesPerWeek(pace)} a week · about ${m.weeksLeft} ${
                m.weeksLeft === 1 ? "week" : "weeks"
              }`
            : "Enter a pace above."
        }
      />
    </>
  );
}

/** Finish date in, required pace out. */
function FromDate({
  ids,
  settings,
  remaining,
  onChange,
}: {
  ids: string;
  settings: PaceSettings;
  remaining: number;
  onChange: (patch: Partial<PaceSettings>) => void;
}) {
  const { pace, target } = settings;

  // Parsed as local midnight. `new Date("2026-12-01")` is parsed as UTC, which
  // lands on the previous day for anyone west of Greenwich — a deadline card
  // that is a day out is worse than no deadline card.
  const parsed = target ? new Date(`${target}T00:00:00`) : null;
  const needed =
    parsed && !Number.isNaN(parsed.getTime())
      ? paceForDeadline(remaining, parsed, pace.daysPerWeek)
      : null;

  return (
    <>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <label htmlFor={`${ids}-target`} className="text-sm font-medium text-ink">
            Finish by
          </label>
          <input
            id={`${ids}-target`}
            type="date"
            value={target}
            onChange={(e) => onChange({ target: e.target.value })}
            className="w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm
              text-ink transition-colors focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>
        <NumberField
          id={`${ids}-days-2`}
          label="Study days a week"
          value={pace.daysPerWeek}
          min={1}
          max={7}
          onChange={(daysPerWeek) => onChange({ pace: { ...pace, daysPerWeek } })}
        />
      </div>

      {needed?.tooLate ? (
        <Outcome
          lead="That date has gone"
          headline={`All ${remaining} today`}
          detail="Pick a date in the future to get a workable pace."
          warn
        />
      ) : (
        <Outcome
          lead="You would need"
          headline={needed ? `${formatRate(needed.perDay)} a study day` : "—"}
          detail={
            needed
              ? `${remaining} left · ${formatRate(needed.perWeek)} a week · ${
                  needed.daysAvailable
                } ${needed.daysAvailable === 1 ? "day" : "days"} from today`
              : "Pick a date above."
          }
        />
      )}
    </>
  );
}

function CardLabel() {
  return (
    <p className="text-xs font-medium uppercase tracking-wider text-ink-muted">
      Your pace
    </p>
  );
}

function ModeButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`rounded-md px-3 py-1 text-xs font-medium transition-colors
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent ${
          active ? "bg-accent-soft text-accent" : "text-ink-muted hover:text-ink"
        }`}
    >
      {children}
    </button>
  );
}

/**
 * A whole-number field.
 *
 * An empty box mid-edit is ignored rather than treated as zero: clearing the
 * field to type a new number briefly makes it invalid, and a control that
 * snapped back to a clamped value on the way would fight the person using it.
 */
function NumberField({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-ink">
        {label}
      </label>
      <input
        id={id}
        type="number"
        inputMode="numeric"
        min={min}
        max={max}
        step={1}
        value={value}
        onChange={(e) => {
          const n = Number(e.target.value);
          if (e.target.value === "" || !Number.isFinite(n)) return;
          onChange(Math.min(max, Math.max(min, Math.round(n))));
        }}
        className="w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm
          text-ink transition-colors focus:outline-none focus:ring-2 focus:ring-accent"
      />
    </div>
  );
}

function Outcome({
  lead,
  headline,
  detail,
  warn = false,
}: {
  lead: string;
  headline: string;
  detail: string;
  warn?: boolean;
}) {
  return (
    <div className="mt-4" role="status">
      <p className="text-xs text-ink-muted">{lead}</p>
      <p
        className={`mt-0.5 font-serif text-2xl font-semibold ${
          warn ? "text-warning" : "text-ink"
        }`}
      >
        {headline}
      </p>
      <p className="mt-1 text-xs text-ink-muted">{detail}</p>
    </div>
  );
}
