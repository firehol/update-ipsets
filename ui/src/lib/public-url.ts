/**
 * Builds links to the public website from admin views.
 *
 * publicBaseURL is the externally visible base URL of the public
 * site and may include a path prefix (for example
 * https://example.test/iplists). When it is unset we fall back to
 * same-origin relative paths, which keeps shared-listener local
 * development ergonomic.
 */
export function publicSiteURL(
  publicBaseURL: string | null | undefined,
  path: string,
): string | null {
  const cleanPath = path.replace(/^\/+/, "");
  if (publicBaseURL === undefined) {
    return null;
  }
  const trimmedBase = publicBaseURL?.trim();
  if (!trimmedBase) {
    return `/${cleanPath}`;
  }

  const url = new URL(trimmedBase);
  const basePath = url.pathname.endsWith("/")
    ? url.pathname
    : `${url.pathname}/`;
  url.pathname = `${basePath}${cleanPath}`.replace(/\/{2,}/g, "/");
  url.search = "";
  url.hash = "";
  return url.toString();
}

export function publicFeedURL(
  publicBaseURL: string | null | undefined,
  name: string,
): string | null {
  return publicSiteURL(publicBaseURL, `ipsets/${encodeURIComponent(name)}`);
}
