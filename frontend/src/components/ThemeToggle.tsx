import { useEffect, useState } from "react";

const KEY = "excelplan:theme";

/**
 * Light/dark, remembered.
 *
 * Defaults to the system preference rather than to light, so the first paint
 * matches the rest of the machine instead of flashing white at somebody
 * working at night.
 */
export function ThemeToggle() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    let stored: string | null = null;
    try {
      stored = localStorage.getItem(KEY);
    } catch {
      /* private mode */
    }
    const prefersDark =
      stored === null
        ? window.matchMedia("(prefers-color-scheme: dark)").matches
        : stored === "dark";
    setDark(prefersDark);
    document.documentElement.classList.toggle("dark", prefersDark);
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    try {
      localStorage.setItem(KEY, next ? "dark" : "light");
    } catch {
      /* private mode */
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      className="rounded-lg border border-line px-3 py-1.5 text-sm text-ink-muted
        transition-colors hover:text-ink focus-visible:outline-none
        focus-visible:ring-2 focus-visible:ring-accent"
      aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
    >
      {dark ? "Light" : "Dark"}
    </button>
  );
}
