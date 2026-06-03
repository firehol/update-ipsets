import DOMPurify from "dompurify";

/**
 * sanitizeHtml runs an HTML fragment through DOMPurify before React renders
 * the allowed nodes. Current content sources are first-party, but sanitizing
 * here keeps future content-source changes from silently expanding trust.
 */
export function sanitizeHtml(input: string | null | undefined): string {
  if (!input) return "";
  return DOMPurify.sanitize(input, {
    USE_PROFILES: { html: true },
  });
}
