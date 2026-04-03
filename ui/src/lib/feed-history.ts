export interface HistoryPoint {
  ts: number;
  ips: number;
  entries: number;
}

/**
 * Parses the public `<feed>_history.csv` payload into millisecond-based chart
 * points. Shared by the hero evolution chart and the behavior section so both
 * surfaces speak from the exact same history series.
 */
export function parseHistoryCSV(csv: string | undefined): HistoryPoint[] {
  if (!csv) return [];
  const lines = csv.trim().split("\n");
  if (lines.length < 2) return [];
  const out: HistoryPoint[] = [];
  for (let i = 1; i < lines.length; i++) {
    const parts = lines[i].split(",");
    if (parts.length < 3) continue;
    const ts = parseInt(parts[0], 10) * 1000;
    const entries = parseInt(parts[1], 10);
    const ips = parseInt(parts[2], 10);
    if (Number.isNaN(ts) || Number.isNaN(entries) || Number.isNaN(ips)) continue;
    out.push({ ts, ips, entries });
  }
  return out.slice(-500);
}
