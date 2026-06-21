export const LIVE_QUEUE_VIEWPORT_CLASS = "h-56 overflow-y-auto";
export const LIVE_QUEUE_TILE_CLASS =
  "grid h-[17.55rem] min-w-0 grid-rows-[auto_minmax(0,1fr)] bg-card";
export const LIVE_QUEUE_TILE_VIEWPORT_CLASS = "min-h-0 overflow-y-auto";
export const LIVE_QUEUE_EMPTY_CLASS =
  "flex h-full items-center justify-center px-6 py-8 text-center text-sm text-muted-foreground";

export function parseGoTime(s: string | undefined): number {
  if (!s || s.startsWith("0001-01-01")) return 0;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return 0;
  return Math.floor(d.getTime() / 1000);
}
