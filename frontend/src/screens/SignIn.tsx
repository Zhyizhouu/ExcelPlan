import { useState } from "react";
import { useSession } from "../app/session";
import { Button, Input } from "../components/ui";
import { ThemeToggle } from "../components/ThemeToggle";

/**
 * The split-pane sign-in ESdemy uses: a dark showcase panel carrying the
 * identity, a light form doing the work.
 *
 * The showcase side states plainly that this is a local tool with no account
 * behind it. A login screen that looks real and is not invites someone to
 * assume it protects something.
 */
export function SignIn() {
  const { signIn } = useSession();
  const [name, setName] = useState("");

  return (
    <div className="min-h-screen w-full bg-canvas lg:grid lg:grid-cols-2">
      <aside
        className="relative hidden flex-col justify-between overflow-hidden p-10 text-white lg:flex"
        style={{
          background:
            "radial-gradient(circle at 20% 20%, rgba(122,184,148,0.22), transparent 44%)," +
            "radial-gradient(circle at 80% 62%, rgba(166,128,40,0.14), transparent 46%)," +
            "linear-gradient(160deg, #16241c 0%, #101a14 58%, #0b120e 100%)",
        }}
      >
        <span className="font-serif text-sm font-semibold tracking-[0.22em]">
          EXCELPLAN
        </span>

        <div className="max-w-md">
          <h2 className="font-serif text-4xl font-semibold leading-tight">
            One case a day.
          </h2>
          <p className="mt-3 text-white/70">
            110 cases across 22 weeks, from keyboard speed to DAX measures — tracked
            straight from the notes you already keep.
          </p>
        </div>

        <p className="text-xs leading-relaxed text-white/45">
          A local tool. Your progress is read from the Obsidian vault on this
          machine and never leaves it — there is no account and nothing to sign
          up for.
        </p>
      </aside>

      <main className="flex min-h-screen flex-col px-6 py-10 sm:px-12 lg:px-16">
        <div className="mb-8 flex items-center justify-between lg:justify-end">
          <span className="font-serif text-sm font-semibold tracking-[0.22em] text-ink lg:hidden">
            EXCELPLAN
          </span>
          <ThemeToggle />
        </div>

        <div className="flex flex-1 flex-col justify-center">
          <div className="mx-auto w-full max-w-sm">
            <h1 className="text-3xl font-semibold text-ink">Welcome back</h1>
            <p className="mt-2 text-sm text-ink-muted">
              Pick a name to greet you by. Nothing is checked or stored anywhere
              but this browser.
            </p>

            <form
              className="mt-8 space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                signIn(name);
              }}
            >
              <Input
                id="signin-name"
                label="Your name"
                placeholder="Alfred"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoComplete="given-name"
              />
              <Button type="submit" className="w-full">
                Open my plan
              </Button>
            </form>
          </div>
        </div>
      </main>
    </div>
  );
}
