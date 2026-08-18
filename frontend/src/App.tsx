import { useCallback, useEffect, useState } from "react";
import { ApiError, fetchProgress, setComplete, type Progress } from "./api/progress";
import { SessionProvider, useSession } from "./app/session";
import { Button, Card, Skeleton } from "./components/ui";
import { ThemeToggle } from "./components/ThemeToggle";
import { CaseDetail } from "./screens/CaseDetail";
import { Dashboard } from "./screens/Dashboard";
import { Phases } from "./screens/Phases";
import { SignIn } from "./screens/SignIn";

type Tab = "dashboard" | "phases";

export default function App() {
  return (
    <SessionProvider>
      <Root />
    </SessionProvider>
  );
}

function Root() {
  const { name } = useSession();
  return name ? <Shell /> : <SignIn />;
}

function Shell() {
  const { name, signOut } = useSession();
  const [tab, setTab] = useState<Tab>("dashboard");
  const [progress, setProgress] = useState<Progress | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [loading, setLoading] = useState(true);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const [slugError, setSlugError] = useState<{ slug: string; message: string } | null>(
    null,
  );
  /** Non-null when a case detail page is open, over whichever tab is behind it. */
  const [openSlug, setOpenSlug] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setProgress(await fetchProgress());
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(String(err)));
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Tick a case, writing through to the note.
   *
   * Optimistic, then authoritative: the checkbox flips at once because a
   * round trip to disk makes a checkbox feel broken, and the server's
   * response — the whole recomputed plan — replaces the guess a moment later.
   * A failure rolls the guess back and says so next to that specific case,
   * rather than as a page-level banner that does not tell you which tick was
   * lost.
   */
  const toggle = useCallback(
    async (slug: string, complete: boolean) => {
      setBusySlug(slug);
      setSlugError(null);

      const before = progress;
      setProgress((prev) => (prev ? applyLocally(prev, slug, complete) : prev));

      try {
        setProgress(await setComplete(slug, complete));
      } catch (err) {
        setProgress(before); // the file did not change, so neither should the screen
        setSlugError({
          slug,
          message: err instanceof ApiError ? err.message : String(err),
        });
      } finally {
        setBusySlug(null);
      }
    },
    [progress],
  );

  useEffect(() => {
    void load();
  }, [load]);

  // The vault is edited in Obsidian while this page sits open, so a box ticked
  // in the other window should show up here without a manual refresh.
  // Refetching on focus buys that with no polling loop and no socket, and it
  // fires exactly when the answer might have changed — when you come back.
  useEffect(() => {
    const onFocus = () => void load();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [load]);

  return (
    <div className="flex min-h-screen flex-col bg-canvas">
      <header className="border-b border-line">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
          <div className="flex items-center gap-6">
            <span className="font-serif text-sm font-semibold tracking-[0.22em] text-ink">
              EXCELPLAN
            </span>
            <nav className="flex items-center gap-4" aria-label="Main">
              {(["dashboard", "phases"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => {
                    setOpenSlug(null);
                    setTab(t);
                  }}
                  aria-current={tab === t ? "page" : undefined}
                  className={`text-sm capitalize transition-colors focus-visible:outline-none
                    focus-visible:ring-2 focus-visible:ring-accent ${
                      tab === t
                        ? "font-medium text-ink"
                        : "text-ink-muted hover:text-ink"
                    }`}
                >
                  {t}
                </button>
              ))}
            </nav>
          </div>

          <div className="flex items-center gap-3">
            <ThemeToggle />
            <button
              type="button"
              onClick={signOut}
              className="text-sm text-ink-muted transition-colors hover:text-ink"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">
        {loading && !progress && !error && <LoadingState />}

        {error && (
          <Card className="border-warning/40">
            <h2 className="text-lg font-semibold text-ink">
              {error.offline ? "The server isn't running" : "Couldn't read the plan"}
            </h2>
            <p className="mt-2 text-sm text-ink-muted">{error.message}</p>
            <Button variant="secondary" className="mt-4" onClick={() => void load()}>
              Try again
            </Button>
          </Card>
        )}

        {openSlug ? (
          <CaseDetail
            slug={openSlug}
            complete={completionOf(progress, openSlug)}
            onBack={() => setOpenSlug(null)}
            onToggle={toggle}
            busy={busySlug === openSlug}
            error={slugError?.slug === openSlug ? slugError.message : null}
          />
        ) : (
          progress &&
          !error &&
          (tab === "dashboard" ? (
            <Dashboard
              progress={progress}
              onOpenPhases={() => setTab("phases")}
              onOpenCase={setOpenSlug}
            />
          ) : (
            <Phases
              progress={progress}
              onToggle={toggle}
              onOpen={setOpenSlug}
              busySlug={busySlug}
              slugError={slugError}
            />
          ))
        )}
      </main>

      <footer className="border-t border-line py-6">
        <p className="mx-auto max-w-5xl px-4 text-xs text-ink-muted">
          Reading <span className="font-mono">Fred\Excel Mastery</span> · signed in
          as {name}
        </p>
      </footer>
    </div>
  );
}

/**
 * Whether one case is ticked, according to the plan the server last sent.
 *
 * The detail page fetches its own copy of a note when it opens, and that copy
 * is a snapshot — nothing updates it when the case is ticked from the page
 * itself, so its checkbox would sit unticked until a reload while the vault,
 * the dashboard and every other screen already disagreed. Reading completion
 * from the shared plan instead gives the tick one home, and the same optimistic
 * update every other screen gets, for free.
 *
 * Undefined while the plan is still loading, or for a slug it does not contain;
 * the page then falls back to what its own fetch returned.
 */
function completionOf(progress: Progress | null, slug: string): boolean | undefined {
  for (const phase of progress?.phases ?? []) {
    for (const week of phase.weeks) {
      for (const note of week.notes) {
        if (note.Slug === slug) return note.Complete;
      }
    }
  }
  return undefined;
}

/**
 * Recompute the plan as if one case had been ticked.
 *
 * Only used to fill the gap between the click and the server's answer, which
 * then replaces it wholesale. It deliberately does *not* recompute the streak
 * or which case is next — those depend on dates and ordering the server owns,
 * and a plausible-looking guess that turns out wrong is worse than the value
 * simply not moving for a moment.
 */
function applyLocally(progress: Progress, slug: string, complete: boolean): Progress {
  let delta = 0;

  const phases = progress.phases.map((phase) => {
    let phaseDelta = 0;

    const weeks = phase.weeks.map((week) => {
      let weekDelta = 0;
      const notes = week.notes.map((note) => {
        if (note.Slug !== slug || note.Complete === complete) return note;
        weekDelta += complete ? 1 : -1;
        return { ...note, Complete: complete };
      });
      if (weekDelta === 0) return week;
      phaseDelta += weekDelta;
      return { ...week, notes, completed: week.completed + weekDelta };
    });

    if (phaseDelta === 0) return phase;
    delta += phaseDelta;
    return { ...phase, weeks, completed: phase.completed + phaseDelta };
  });

  if (delta === 0) return progress;
  return { ...progress, phases, completed: progress.completed + delta };
}

function LoadingState() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-10 w-64" />
      <Skeleton className="h-40 w-full" />
      <div className="grid gap-4 sm:grid-cols-3">
        <Skeleton className="h-28" />
        <Skeleton className="h-28" />
        <Skeleton className="h-28" />
      </div>
    </div>
  );
}
