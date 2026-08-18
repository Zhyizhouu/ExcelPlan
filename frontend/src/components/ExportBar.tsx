import { useState } from "react";
import {
  ApiError,
  fetchCsv,
  saveWorkbook,
  workbookUrl,
  type SavedWorkbook,
} from "../api/progress";
import { Button } from "./ui";

type Status = { kind: "idle" | "ok" | "error"; message?: string };

/**
 * The four ways to get a case's data into Excel.
 *
 * **Open in Excel** is the primary one, and the whole path from "read the
 * brief" to "start working" in one click: it puts the workbook in the vault
 * beside its case note if it isn't there yet, then opens it. Coming back to a
 * case you already started opens the workbook you already have — with your
 * answers in it — because the file is never written over.
 *
 * **Save to vault** is the same thing without the launch, for stocking up a
 * few cases ahead of a session you are not starting yet.
 *
 * Both leave the answer somewhere it will still be tomorrow. **Download** and
 * **Copy** are for working somewhere else — another machine, a sheet already
 * open — and a download lands in the Downloads folder, away from the note it
 * belongs to, where it gets lost.
 *
 * Every outcome is stated in words. These actions touch the filesystem and the
 * clipboard — neither of which shows its own result inside the page — so a
 * button that silently succeeds is indistinguishable from one that silently
 * failed.
 */
export function ExportBar({ slug }: { slug: string }) {
  const [copy, setCopy] = useState<Status>({ kind: "idle" });
  const [save, setSave] = useState<Status>({ kind: "idle" });
  /** Which of the two vault buttons is mid-flight, so only that one says so. */
  const [busy, setBusy] = useState<"open" | "save" | null>(null);

  async function onCopy() {
    setCopy({ kind: "idle" });
    try {
      const csv = await fetchCsv(slug);
      await navigator.clipboard.writeText(csv);
      const rows = csv.trimEnd().split("\n").length - 1; // minus the header
      setCopy({ kind: "ok", message: `${rows} rows copied — paste into A1.` });
    } catch (err) {
      // Clipboard writes are refused outright in some contexts, which is a
      // different problem from the fetch failing and needs saying differently.
      const message =
        err instanceof ApiError
          ? err.message
          : "The browser blocked the clipboard. Use Download instead.";
      setCopy({ kind: "error", message });
    }
  }

  async function onSave(open: boolean) {
    setBusy(open ? "open" : "save");
    setSave({ kind: "idle" });
    try {
      setSave({ kind: "ok", message: saveMessage(await saveWorkbook(slug, open), open) });
    } catch (err) {
      setSave({
        kind: "error",
        message: err instanceof ApiError ? err.message : String(err),
      });
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        {/*
          Labelled for what it does rather than how: on a case you have already
          started it opens your workbook and writes nothing. "Save & open"
          would read, on that second visit, like it had just overwritten the
          answers — the one thing it is built never to do.
        */}
        <Button onClick={() => void onSave(true)} disabled={busy !== null}>
          {busy === "open" ? "Opening…" : "Open in Excel"}
        </Button>

        <Button
          variant="secondary"
          onClick={() => void onSave(false)}
          disabled={busy !== null}
        >
          {busy === "save" ? "Saving…" : "Save to vault"}
        </Button>

        {/*
          A real anchor, not a fetch-and-blob. The browser then uses the
          server's Content-Disposition filename, so the download is named
          "W1D1 - Keyboard only - Answer.xlsx" without the client having to
          reconstruct that name and get it subtly wrong.

          Styled by hand to match Button's secondary variant, because Button
          renders a <button> and a download has to be an anchor.
        */}
        <a
          href={workbookUrl(slug)}
          download
          className="rounded-lg border border-line bg-surface px-4 py-2 text-sm
            font-medium text-ink transition-colors hover:border-accent
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent
            focus-visible:ring-offset-2 focus-visible:ring-offset-canvas"
        >
          Download .xlsx
        </a>

        <Button variant="secondary" onClick={() => void onCopy()}>
          Copy as CSV
        </Button>
      </div>

      <Note status={save} />
      <Note status={copy} />

      <p className="text-xs text-ink-muted">
        Both write <span className="font-mono">&lt;case&gt; - Answer.xlsx</span> next to
        the case note. Neither ever overwrites one already there — come back to a
        case and you get the workbook you left, answers and all.
      </p>
    </div>
  );
}

/**
 * What just happened to the file, in one sentence.
 *
 * "Saved" and "already there" have to read differently: one is a new file, the
 * other is your existing work being handed back untouched, and someone about to
 * type into it should know which they are looking at.
 */
function saveMessage(result: SavedWorkbook, wantedOpen: boolean): string {
  const where = result.created
    ? `Saved to ${result.path}`
    : `Already in the vault — ${result.path}, untouched`;

  if (!wantedOpen) return `${where}.`;
  if (result.opened) return `${where}. Opening in Excel…`;
  return `${where}. Excel didn't open on its own — open it from the vault folder.`;
}

function Note({ status }: { status: Status }) {
  if (status.kind === "idle") return null;
  return (
    <p
      role="status"
      className={`text-xs ${status.kind === "ok" ? "text-accent" : "text-warning"}`}
    >
      {status.message}
    </p>
  );
}
