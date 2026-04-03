const LINK_RE = /\[([^\]]+)\]\(([^)]+)\)/g;
const BOLD_RE = /\*\*([^*]+)\*\*/g;

/**
 * Extract the most quotable sentence from a markdown blob. Used to fill
 * the pull-quote callout in the editorial section. Preference:
 *   - longest sentence in the second paragraph (typically the methodology line)
 *   - fall back to the longest sentence overall
 *   - return null if the input is empty or too short to quote
 */
export function extractPullQuote(value: string | null | undefined): string | null {
  const trimmed = value?.trim();
  if (!trimmed) return null;
  const paragraphs = trimmed.split(/\n\s*\n/).map((p) => p.trim()).filter(Boolean);
  if (paragraphs.length === 0) return null;
  const candidates = paragraphs[1] ? splitSentences(paragraphs[1]) : [];
  const fallback = paragraphs.flatMap(splitSentences);
  const pool = candidates.length > 0 ? candidates : fallback;
  const usable = pool.filter((s) => s.length >= 40 && s.length <= 220);
  if (usable.length === 0) return null;
  return usable.sort((a, b) => b.length - a.length)[0]
    .replace(/^\s*[-*]\s*/, "")
    .replace(LINK_RE, "$1")
    .replace(BOLD_RE, "$1");
}

function splitSentences(paragraph: string): string[] {
  return paragraph
    .replace(/\s+/g, " ")
    .split(/(?<=[.!?])\s+(?=[A-Z“"'])/)
    .map((s) => s.trim())
    .filter(Boolean);
}
