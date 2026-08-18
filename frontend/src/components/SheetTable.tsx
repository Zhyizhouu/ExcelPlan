/**
 * A table that looks like a spreadsheet, because the thing being taught is a
 * spreadsheet.
 *
 * Column letters and row numbers are not decoration: the cases talk in terms
 * of `=LEFT(A2,4)` and "Ctrl+Shift+↓", and a plain HTML table gives the reader
 * nothing to map that onto. With the gutters, the formula and the picture
 * agree.
 *
 * Row 1 is the header row, exactly as it would be in a real sheet, so `A2` in
 * a formula points at the first data row here too.
 */

const LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";

function columnLetter(index: number): string {
  // AA, AB… past 26 columns. None of the datasets get that wide, but a table
  // that silently mislabels column 27 is worse than one that handles it.
  let out = "";
  let n = index;
  while (n >= 0) {
    out = LETTERS[n % 26] + out;
    n = Math.floor(n / 26) - 1;
  }
  return out;
}

export interface SheetData {
  columns: string[];
  rows: Array<Record<string, unknown>>;
}

export function SheetTable({
  data,
  maxRows,
  caption,
  highlight = [],
}: {
  data: SheetData;
  /** Truncate long sheets. The footer says how many were hidden. */
  maxRows?: number;
  caption?: string;
  /** Column names to tint — the ones this case is actually about. */
  highlight?: string[];
}) {
  const shown = maxRows ? data.rows.slice(0, maxRows) : data.rows;
  const hidden = data.rows.length - shown.length;

  return (
    <figure className="min-w-0">
      {caption && (
        <figcaption className="mb-1.5 text-xs text-ink-muted">{caption}</figcaption>
      )}

      {/* The scroller is the wide element, not the page. */}
      <div className="overflow-x-auto rounded-lg border border-line bg-surface">
        <table className="w-full border-collapse font-mono text-xs">
          <thead>
            {/* The column-letter gutter. */}
            <tr>
              <th className="sticky left-0 z-10 w-10 border-b border-r border-line bg-canvas
                px-2 py-1 text-center font-normal text-ink-muted">
                &nbsp;
              </th>
              {data.columns.map((col, i) => (
                <th
                  key={col}
                  className={`border-b border-r border-line px-2 py-1 text-center
                    font-normal text-ink-muted last:border-r-0 ${
                      highlight.includes(col) ? "bg-accent-soft" : "bg-canvas"
                    }`}
                >
                  {columnLetter(i)}
                </th>
              ))}
            </tr>
            {/* Row 1: the real header row. */}
            <tr>
              <th className="sticky left-0 z-10 border-b border-r border-line bg-canvas
                px-2 py-1 text-center font-normal text-ink-muted">
                1
              </th>
              {data.columns.map((col) => (
                <th
                  key={col}
                  className={`whitespace-nowrap border-b border-r border-line px-2 py-1
                    text-left font-semibold text-ink last:border-r-0 ${
                      highlight.includes(col) ? "bg-accent-soft" : ""
                    }`}
                >
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {shown.map((row, r) => (
              <tr key={r} className="even:bg-canvas/40">
                <td className="sticky left-0 z-10 border-b border-r border-line bg-canvas
                  px-2 py-1 text-center text-ink-muted">
                  {r + 2}
                </td>
                {data.columns.map((col) => {
                  const value = row[col];
                  const numeric = typeof value === "number";
                  return (
                    <td
                      key={col}
                      className={`whitespace-nowrap border-b border-r border-line px-2 py-1
                        text-ink last:border-r-0 ${numeric ? "text-right" : "text-left"} ${
                          highlight.includes(col) ? "bg-accent-soft/50" : ""
                        }`}
                    >
                      {value === "" || value === null || value === undefined ? (
                        // A blank cell is data, not a gap — COUNTA vs COUNT
                        // turns on it — so it gets a visible placeholder.
                        <span className="text-ink-muted/40">·</span>
                      ) : (
                        String(value)
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {hidden > 0 && (
        <p className="mt-1.5 text-xs text-ink-muted">
          Showing {shown.length} of {data.rows.length} rows — {hidden} more in the
          sheet.
        </p>
      )}
    </figure>
  );
}
