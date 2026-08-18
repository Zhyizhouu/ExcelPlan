import { useMemo } from "react";
import type { Progress } from "../api/progress";
import { statusOf } from "../api/progress";
import { Badge, Button, Card, ProgressBar } from "../components/ui";
import { formatDate, momentum, takeNewlyComplete } from "../lib/momentum";
import { useSession } from "../app/session";

/**
 * The dashboard answers one question first — what do I do today — and only
 * then shows how far along everything is.
 *
 * Ordering is the whole design. A grid of 110 cases is a decision to make
 * before any work starts, and deciding is the step people skip a day over.
 * The next case is therefore the largest thing on the screen, already chosen.
 */
export function Dashboard({
  progress,
  onOpenPhases,
  onOpenCase,
}: {
  progress: Progress;
  onOpenPhases: () => void;
  onOpenCase: (slug: string) => void;
}) {
  const { name } = useSession();
  const m = useMemo(() => momentum(progress), [progress]);

  // Read once per mount: this both reports and records, so calling it during
  // render of a memo would re-fire on every re-render and swallow the banner.
  const justFinished = useMemo(() => takeNewlyComplete(progress), [progress]);

  const done = progress.completed >= progress.total && progress.total > 0;

  return (
    <div className="space-y-6">
      {justFinished.length > 0 && (
        <Card className="ep-rise border-accent/40 bg-accent-soft">
          <p className="font-serif text-lg text-accent">
            {justFinished.length === 1
              ? `Phase ${justFinished[0]} complete.`
              : `Phases ${justFinished.join(", ")} complete.`}
          </p>
          <p className="mt-1 text-sm text-ink-muted">
            That is a whole section of the roadmap behind you.
          </p>
        </Card>
      )}

      <header>
        <h1 className="text-3xl font-semibold text-ink">
          {name ? `Welcome back, ${name}` : "Your plan"}
        </h1>
        <p className="mt-1 text-sm text-ink-muted">
          Excel Mastery — {progress.total} cases, five a week, from calibration to
          the ops capstone.
        </p>
      </header>

      {/* Next up: the single most useful thing on the page. */}
      {progress.nextUp ? (
        <Card className="border-accent/30">
          <p className="text-xs font-medium uppercase tracking-wider text-ink-muted">
            Next up
          </p>
          <h2 className="mt-2 text-2xl font-semibold text-ink">
            {progress.nextUp.Title || progress.nextUp.Slug}
          </h2>
          <p className="mt-1 font-mono text-sm text-ink-muted">
            {progress.nextUp.Slug} · Phase {progress.nextUp.Phase} · Week{" "}
            {progress.nextUp.Week}
          </p>

          {progress.nextUp.Fields && progress.nextUp.Fields.length > 0 && (
            <dl className="mt-4 space-y-2 border-l border-line pl-4">
              {progress.nextUp.Fields.map((field) => (
                <div key={field.label}>
                  <dt className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
                    {field.label}
                  </dt>
                  <dd className="mt-0.5 text-sm leading-relaxed text-ink">
                    {field.value}
                  </dd>
                </div>
              ))}
            </dl>
          )}

          <div className="mt-5 flex flex-wrap gap-2">
            <Button onClick={() => onOpenCase(progress.nextUp!.Slug)}>
              Open this case
            </Button>
            <Button variant="secondary" onClick={onOpenPhases}>
              See all phases
            </Button>
          </div>
        </Card>
      ) : (
        <Card className="border-accent/40 bg-accent-soft">
          <h2 className="font-serif text-2xl text-accent">All 110 cases complete.</h2>
          <p className="mt-1 text-sm text-ink-muted">
            The whole roadmap, start to finish.
          </p>
        </Card>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat
          label="Overall"
          value={`${Math.round(m.percent)}%`}
          detail={`${progress.completed} of ${progress.total} cases`}
        >
          <ProgressBar
            value={progress.completed}
            total={progress.total}
            label="Overall progress"
            className="mt-3"
          />
        </Stat>

        <Stat
          label="Streak"
          value={
            progress.streak.current > 0
              ? `${progress.streak.current} ${progress.streak.current === 1 ? "day" : "days"}`
              : "—"
          }
          detail={
            progress.streak.longest > 0
              ? `Best: ${progress.streak.longest} days`
              : "Tick a case to start one"
          }
        />

        <Stat
          label={done ? "Finished" : "On track for"}
          value={done ? "Done" : m.finishDate ? formatDate(m.finishDate) : "—"}
          detail={
            done
              ? "Every phase closed"
              : `${m.remaining} left · about ${m.weeksLeft} ${m.weeksLeft === 1 ? "week" : "weeks"} at five a week`
          }
        />
      </div>

      <section>
        <h2 className="mb-3 text-lg font-semibold text-ink">Phases</h2>
        <div className="space-y-3">
          {progress.phases.map((phase) => (
            <Card key={phase.key} className="flex flex-col gap-3">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <h3 className="text-base font-semibold text-ink">
                    Phase {phase.key}
                    {phase.name ? ` — ${phase.name}` : ""}
                  </h3>
                  <p className="text-xs text-ink-muted">{phase.weekRange}</p>
                </div>
                <Badge status={statusOf(phase.completed, phase.total)} />
              </div>

              <div className="flex items-center justify-between text-xs text-ink-muted">
                <span>
                  {phase.total === 0
                    ? "0%"
                    : `${Math.round((phase.completed / phase.total) * 100)}%`}
                </span>
                <span className="font-mono">
                  {phase.completed}/{phase.total}
                </span>
              </div>
              <ProgressBar
                value={phase.completed}
                total={phase.total}
                label={`Phase ${phase.key}`}
              />
            </Card>
          ))}
        </div>
      </section>
    </div>
  );
}

function Stat({
  label,
  value,
  detail,
  children,
}: {
  label: string;
  value: string;
  detail: string;
  children?: React.ReactNode;
}) {
  return (
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-ink-muted">
        {label}
      </p>
      <p className="mt-2 font-serif text-2xl font-semibold text-ink">{value}</p>
      <p className="mt-1 text-xs text-ink-muted">{detail}</p>
      {children}
    </Card>
  );
}
