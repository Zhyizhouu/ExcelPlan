/**
 * The small kit every screen is built from — the same Base* set ESdemy uses,
 * as one file because there are six of them and six files would be filing for
 * its own sake.
 */

import type { ReactNode } from "react";
import type { Status } from "../api/progress";

export function Card({
  children,
  className = "",
  as: Tag = "div",
  ...rest
}: {
  children: ReactNode;
  className?: string;
  as?: "div" | "article" | "section";
} & React.HTMLAttributes<HTMLElement>) {
  return (
    <Tag
      className={`rounded-card border border-line bg-surface p-5 ${className}`}
      {...rest}
    >
      {children}
    </Tag>
  );
}

const badgeStyles: Record<Status, string> = {
  not_started: "border-line text-ink-muted",
  in_progress: "border-warning/40 text-warning",
  complete: "border-accent/40 bg-accent-soft text-accent",
};

export const statusLabel: Record<Status, string> = {
  not_started: "Not started",
  in_progress: "In progress",
  complete: "Complete",
};

export function Badge({ status }: { status: Status }) {
  return (
    <span
      className={`shrink-0 rounded-full border px-2.5 py-0.5 text-xs font-medium ${badgeStyles[status]}`}
    >
      {statusLabel[status]}
    </span>
  );
}

/**
 * A progress bar that states its own value to assistive tech.
 *
 * The visible percentage is nearby in every use, but the bar is the thing
 * that reads as "progress", so it carries the ARIA role rather than relying on
 * a sighted reader pairing it with adjacent text.
 */
export function ProgressBar({
  value,
  total,
  label,
  className = "",
}: {
  value: number;
  total: number;
  label: string;
  className?: string;
}) {
  const pct = total === 0 ? 0 : Math.round((value / total) * 100);
  return (
    <div
      className={`h-1.5 w-full overflow-hidden rounded-full bg-line ${className}`}
      role="progressbar"
      aria-valuenow={value}
      aria-valuemin={0}
      aria-valuemax={total}
      aria-label={`${label}: ${value} of ${total} complete`}
    >
      <div
        className="h-full rounded-full bg-accent transition-[width] duration-500"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

export function Button({
  children,
  variant = "primary",
  className = "",
  ...rest
}: {
  children: ReactNode;
  variant?: "primary" | "secondary" | "quiet";
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const styles = {
    primary: "bg-accent text-white hover:bg-accent-hover border-transparent",
    secondary: "bg-surface text-ink border-line hover:border-accent",
    quiet: "bg-transparent text-ink-muted border-transparent hover:text-ink",
  }[variant];

  return (
    <button
      className={`rounded-lg border px-4 py-2 text-sm font-medium transition-colors
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent
        focus-visible:ring-offset-2 focus-visible:ring-offset-canvas
        disabled:cursor-not-allowed disabled:opacity-50 ${styles} ${className}`}
      {...rest}
    >
      {children}
    </button>
  );
}

export function Input({
  label,
  id,
  ...rest
}: { label: string; id: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-ink">
        {label}
      </label>
      <input
        id={id}
        className="w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm
          text-ink placeholder:text-ink-muted transition-colors focus:outline-none
          focus:ring-2 focus:ring-accent"
        {...rest}
      />
    </div>
  );
}

/** A loading placeholder shaped like the thing it stands in for. */
export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-card bg-line/60 ${className}`} />;
}
