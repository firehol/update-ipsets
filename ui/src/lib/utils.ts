import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * cn merges Tailwind class names and resolves conflicts. This is the canonical
 * helper used by every shadcn/ui component — keeping it at @/lib/utils means
 * shadcn CLI installations work without modification.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * formatIPs collapses an integer count of IP addresses into a short
 * human-friendly string (e.g. 1_300_000 → "1.3M"). Mirrors the formatIPs
 * helper in pkg/web/static/app.js so the new UI matches the bundled JSON
 * conventions exactly.
 */
export function formatIPs(n: number | null | undefined): string {
  if (n == null) return "—";
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
  return String(n);
}

/**
 * formatNum prints a count with locale-grouped thousands separators.
 * Used for distinct-entity tallies (ASN count, country count, etc.) where
 * the formatIPs short form would lose precision.
 */
export function formatNum(n: number | null | undefined): string {
  if (n == null) return "—";
  return n.toLocaleString();
}

/**
 * formatPct returns a percentage string from a 0-1 ratio. Always two
 * fractional digits to match the rest of the UI.
 */
export function formatPct(ratio: number, digits = 2): string {
  return (ratio * 100).toFixed(digits) + "%";
}

export function formatPercentValue(
  value: number | null | undefined,
  digits = 1,
): string {
  if (value == null) return "—";
  return `${value.toFixed(digits)}%`;
}

/**
 * formatFreq turns minutes into a short human label ("5m", "2h", "1d").
 * Mirrors formatFreq from pkg/web/static/app.js so feed cards display
 * frequencies the same way users are used to.
 */
export function formatFreq(minutes: number | null | undefined): string {
  if (!minutes) return "—";
  if (minutes < 60) return minutes + "m";
  if (minutes < 1440) return Math.round(minutes / 60) + "h";
  return Math.round(minutes / 1440) + "d";
}

/**
 * timeAgo returns a short relative-time label for a Unix timestamp.
 * Accepts either seconds or milliseconds — we autodetect by magnitude
 * (anything below 1e12 is treated as seconds and converted). Returns
 * "—" when the input is missing or zero.
 */
export function timeAgo(input: number | null | undefined): string {
  if (!input) return "—";
  const millis = input < 1e12 ? input * 1000 : input;
  const now = Date.now();
  const delta = now - millis;
  if (delta < 0) return "just now";
  const seconds = Math.floor(delta / 1000);
  if (seconds < 60) return seconds + "s ago";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return minutes + "m ago";
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return hours + "h ago";
  const days = Math.floor(hours / 24);
  if (days < 30) return days + "d ago";
  const months = Math.floor(days / 30);
  if (months < 12) return months + "mo ago";
  return Math.floor(months / 12) + "y ago";
}
