export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export async function fetchJSON<T>(
  url: string,
  init?: RequestInit,
): Promise<T> {
  const requestPath = sameOriginRequestPath(url);
  const response = await fetch(requestPath, init); // nosemgrep: rules.lgpl.javascript.ssrf.rule-node-ssrf - sameOriginRequestPath rejects cross-origin URLs before fetch.
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = await response.json();
      if (body && typeof body === "object" && "error" in body) {
        message = String(body.error);
      }
    } catch {
      /* body was not JSON; keep the status text */
    }
    throw new ApiError(response.status, message);
  }
  return (await response.json()) as T;
}

export async function fetchText(
  url: string,
  init?: RequestInit,
): Promise<string> {
  const requestPath = sameOriginRequestPath(url);
  const response = await fetch(requestPath, init); // nosemgrep: rules.lgpl.javascript.ssrf.rule-node-ssrf - sameOriginRequestPath rejects cross-origin URLs before fetch.
  if (!response.ok) {
    throw new ApiError(response.status, `${response.status} ${response.statusText}`);
  }
  return await response.text();
}

export function signalInit(signal?: AbortSignal): RequestInit | undefined {
  return signal ? { signal } : undefined;
}

function sameOriginRequestPath(url: string): string {
  const origin = globalThis.location?.origin ?? "http://localhost";
  const parsed = new URL(url, origin);
  if (parsed.origin !== origin) {
    throw new Error("API client only accepts same-origin URLs");
  }
  return `${parsed.pathname}${parsed.search}${parsed.hash}`;
}
