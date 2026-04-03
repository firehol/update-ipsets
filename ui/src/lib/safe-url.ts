const SAFE_EXTERNAL_PROTOCOLS = new Set(["http:", "https:"]);

export function safeExternalUrl(
  value: string | null | undefined,
): string | undefined {
  const trimmed = value?.trim();
  if (!trimmed) return undefined;
  try {
    const parsed = new URL(trimmed);
    if (!SAFE_EXTERNAL_PROTOCOLS.has(parsed.protocol)) return undefined;
    return parsed.href;
  } catch {
    return undefined;
  }
}

export function externalUrlLabel(value: string | null | undefined): string {
  const trimmed = value?.trim();
  if (!trimmed) return "";
  const safeURL = safeExternalUrl(trimmed);
  if (!safeURL) return trimmed;
  return new URL(safeURL).host.replace(/^www\./, "");
}
