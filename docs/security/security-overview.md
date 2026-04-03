# Security Overview

You will learn the security design of update-ipsets, how the two surfaces are protected, and what the threat model covers.

## Design principles

update-ipsets follows a fail-closed security model:

- Secure defaults out of the box
- Unsafe modes require explicit opt-in
- Misconfiguration blocks access rather than opening it

## Two surfaces

The daemon exposes two distinct surfaces:

**Public surface** — the website, API, and feed downloads.

- Read-only. No authentication required.
- Serves precomputed artifacts from disk. Public requests do not trigger downloads, processing, or recomputation.
- Rate-limited to prevent abuse.
- No secrets in URLs or logs.

**Admin surface** — the operator dashboard and control API.

- Requires authentication by default.
- Exposes feed status, queue state, integrity findings, and operator actions (recheck, reprocess, enable, disable).
- Available on the same listener as public, or on a separate admin-only listener.

## Public surface protections

- **Rate limiting:** 240 requests/minute per client IP for general API endpoints. 10 requests/minute for IP search endpoints. These are independent limits.
- **Excluded from rate limiting:** `/healthz` and admin endpoints.
- **No secrets in URLs:** Feed data, metadata, and search results never embed credentials.
- **Path traversal protection:** All artifact and file routes validate paths against traversal attacks.

## Admin surface protections

See [Admin Authentication](admin-authentication.md) for the full authentication model.

- Default mode is `required` — HTTP Basic Auth with configured credentials.
- Missing or empty credentials block admin access entirely.
- Disabling auth requires two explicit flags, not one.
- The admin SPA shell itself is protected behind authentication.

## Security considerations by deployment

| Deployment | Recommendation |
|------------|---------------|
| Local development | Use `--admin-auth-mode=disabled` with both flags, on loopback only |
| Staging | Use `required` auth on the default listener |
| Production | Use split listener with admin on localhost, behind a firewall |

See [Production Deployment](production-deployment.md) for the recommended setup.
