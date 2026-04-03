import DOMPurify from "dompurify";

/**
 * sanitizeHtml runs an HTML fragment through DOMPurify before it lands in
 * dangerouslySetInnerHTML. The content sources today are all first-party
 * (YAML-derived feed info, hand-written ipset description fragments shipped
 * in the daemon binary, methodology markdown rendered server-side), so the
 * sanitizer is defence-in-depth rather than an active mitigation. Adding
 * it now means any future content source — user-generated comments, an
 * upstream ingestion, etc. — is automatically safe.
 */
export function sanitizeHtml(input: string | null | undefined): string {
  if (!input) return "";
  return DOMPurify.sanitize(input, {
    USE_PROFILES: { html: true },
  });
}
