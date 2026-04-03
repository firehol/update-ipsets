# SOW-0060 - Trusted Proxy Client IP Policy

## Status

Status: completed

Sub-state: completed

## Requirements

### Purpose

Make client-IP detection safe and predictable for rate limiting, logging, and `/api/v1/client-ip` by trusting forwarded headers only when explicitly configured.

### User Request

Trusted proxy client IP policy — deployment decision with independent config knobs for proxy headers and Cloudflare headers.

### Assistant Understanding

Facts:

- `clientIP()` at `pkg/web/middleware.go:167` trusted `CF-Connecting-IP`, `X-Forwarded-For`, `X-Real-IP` unconditionally.
- 5 callers: rate limiting, panic recovery log, error logs, search rate limiting, `/api/v1/client-ip` endpoint.
- No trust boundary existed — any client could spoof headers.

### Acceptance Criteria

- [x] Client-IP policy defaults to secure (RemoteAddr only)
- [x] `--trust-proxy-headers` enables X-Forwarded-For and X-Real-IP
- [x] `--trust-cloudflare-headers` enables CF-Connecting-IP
- [x] Each independently configurable via CLI flags and YAML
- [x] Rate limiting, logging, and `/api/v1/client-ip` use the same policy
- [x] Tests cover direct, proxy, Cloudflare, both, spoofed, malformed, multi-hop
- [x] Operator docs include deployment checklist with topology decisions
- [x] Deployment guide mentions trusted proxy configuration options

## Implementation

### Code changes

1. **`clientIPResolver` struct** (`pkg/web/middleware.go`): New struct with `trustProxy` and `trustCloudflare` fields. Method `clientIP()` checks headers only when the corresponding trust flag is enabled. CF-Connecting-IP takes precedence over proxy headers when both are enabled.

2. **`web.Options`** (`pkg/web/server.go`): Two new boolean fields: `TrustProxyHeaders`, `TrustCloudflareHeaders`.

3. **`surfaceRoutes`** (`pkg/web/surface_routes.go`): New `resolver` field on the struct, initialized from `opts.TrustProxyHeaders` and `opts.TrustCloudflareHeaders`.

4. **Middleware chain** (`pkg/web/surface_handler.go`): Resolver passed through `logMiddleware`, `recoverMiddleware`, `rateLimitMiddleware`.

5. **Route handlers** (`pkg/web/routes.go`): `handleClientIP`, `writeGlobalSearch`, `writeFeedScopedSearch` all receive resolver via `surfaceRoutes`.

6. **Engine Runtime** (`pkg/engine/runtime.go`): New `TrustProxyHeaders` and `TrustCloudflareHeaders` fields, wired from `config.RuntimeConfig`.

7. **Config** (`pkg/config/config.go`): New YAML fields `trust_proxy_headers` and `trust_cloudflare_headers`.

8. **CLI** (`cmd/update-ipsets/daemon.go`): New flags `--trust-proxy-headers` and `--trust-cloudflare-headers`. Merged with YAML config.

### Test changes

- **`pkg/web/client_ip_resolver_test.go`**: 13 new tests covering all topologies.
- **`pkg/web/client_ip_api_test.go`**: Updated to test both default (secure) and trusted behavior.
- **`pkg/web/feature_test.go`**: Updated header precedence tests to use resolver directly; rate-limit test uses handler with CF trust enabled.
- **`pkg/web/middleware_test.go`**: Updated `recoverMiddleware` call to pass resolver.

### Doc changes

- **`docs/security/production-deployment.md`**: New "Trusted proxy configuration" section with deployment checklist, CLI flags, YAML config, header priority, and security implications.
- **`docs/security/rate-limiting.md`**: Cross-reference to trusted proxy configuration.
- **`.agents/sow/specs/config.md`**: Runtime settings section updated with trusted proxy policy.

## Validation

- `make test` — PASS (all packages)
- `make race` — PASS (all packages)
- `make lint` (`go vet`) — PASS

## Outcome

Client-IP detection is now secure by default. Forwarded headers are ignored unless the operator explicitly enables them via CLI flags or YAML configuration. Two independent knobs control proxy headers and Cloudflare headers. All 5 callers of the old `clientIP()` function now use the resolver with trust checks.

## Artifact Maintenance

- `AGENTS.md`: No changes needed (no workflow or guardrail changes)
- Runtime project skills: No changes needed (no pattern or convention changes)
- Specs: `.agents/sow/specs/config.md` updated with trusted proxy policy under runtime settings
- End-user/operator docs: `docs/security/production-deployment.md` and `docs/security/rate-limiting.md` updated
- End-user/operator skills: No changes needed
- SOW lifecycle: SOW-0060 completed, moved to done/

## Lessons Extracted

- The old `clientIP()` free function was a hidden security assumption. Converting it to a struct method made the trust policy explicit and testable.
- Existing tests that depended on insecure behavior (trusting headers by default) needed careful updating — they were testing the right behavior for the wrong reason.

## Followup

None.

## Analysis

Sources checked:

- `pkg/web/middleware.go`
- `pkg/web/search_api.go`
- `pkg/web/feature_test.go`

Current state:

- Any peer can set `CF-Connecting-IP`, `X-Forwarded-For`, or `X-Real-IP`.

Risks:

- Spoofable rate-limit keys weaken abuse controls.
- Logs and client-IP API responses can show attacker-supplied values.

## Implications And Decisions

### User Decisions (2026-05-02)

1. **This is a deployment decision, not a runtime feature toggle.** The trusted proxy configuration must be a deployment-time setting that operators configure based on their infrastructure topology.

2. **Configuration knobs required:**
   - Enable/disable trusting proxy headers (e.g., `X-Forwarded-For`, `X-Real-IP`).
   - Enable/disable trusting Cloudflare headers (e.g., `CF-Connecting-IP`).
   - Each independently configurable.

3. **Default: secure by default.** When no trusted proxy is configured, `clientIP` must use `RemoteAddr` only. Forwarded headers are ignored.

4. **Deployment documentation must include:**
   - A deployment checklist with these configuration decisions.
   - Guidance for direct-exposure, reverse-proxy, and Cloudflare topologies.
   - Security implications of each mode.

## Pre-Implementation Gate

**Problem**: `clientIP()` at `pkg/web/middleware.go:167` trusts `CF-Connecting-IP`, `X-Forwarded-For`, and `X-Real-IP` unconditionally. Any client can spoof these headers to bypass rate limits, poison logs, or manipulate the `/api/v1/client-ip` response.

**Evidence**: 5 callers of `clientIP()` across the codebase:
- `pkg/web/middleware.go:80` — rate limiting key
- `pkg/web/middleware.go:98` — panic recovery log
- `pkg/web/middleware.go:160,162` — error logs
- `pkg/web/search_api.go:100` — search rate limiting
- `pkg/web/home_detail_api.go:20` — `/api/v1/client-ip` response

**Root cause**: No trust boundary. Headers are accepted without verifying the connection source.

**Affected contracts**:
- `clientIP()` function signature and behavior changes (internal to `pkg/web`)
- `web.Options` struct gets two new boolean fields
- CLI flags in `cmd/update-ipsets/daemon.go`
- Existing tests at `pkg/web/feature_test.go:442-450` that lock in insecure header precedence must be updated
- `RuntimeConfig` in `pkg/config/config.go` may need new YAML fields for config-file support

**Existing patterns to reuse**:
- `web.Options` is the web server config struct, passed through middleware chain
- Config is loaded in `cmd/update-ipsets/daemon.go` and passed to `web.Run()`
- CLI flags map to `web.Options` fields directly

**Risk and blast radius**: Low. All changes are internal to `pkg/web`. No public API contract changes. The `/api/v1/client-ip` endpoint behavior changes only for deployments that don't opt in — it will return `RemoteAddr`-derived IP instead of header-derived IP.

**Sensitive data handling**: No sensitive data in code or docs. Config examples use standard header names.

**Implementation plan**:
1. Add `TrustProxyHeaders bool` and `TrustCloudflareHeaders bool` to `web.Options`
2. Add `--trust-proxy-headers` and `--trust-cloudflare-headers` CLI flags
3. Refactor `clientIP()` to accept options, check flags before reading headers
4. Thread the options through middleware closures
5. Update existing tests to pass flags
6. Add new tests for all topology cases
7. Add `runtime.trust_proxy_headers` and `runtime.trust_cloudflare_headers` YAML config
8. Update spec and operator docs

**Validation plan**:
- `make test`, `make race`, `make lint`
- New tests cover: default (secure), proxy-enabled, CF-enabled, both-enabled, spoofed headers ignored
- Existing tests updated to explicitly enable headers to preserve their current behavior

**Artifact impact plan**:
- `specs/config.md`: add trusted proxy config section
- Operator docs: add deployment checklist
- `AGENTS.md`: no changes needed (no workflow/guardrail changes)
- Project skills: no changes needed

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.

### 2026-05-02

- User recorded decisions: deployment config, independent knobs, secure by default, deployment checklist.
- Implemented `clientIPResolver`, wired through all 5 callers.
- Added CLI flags and YAML config support.
- Added 13 new resolver tests + updated existing tests.
- Updated operator docs and spec.
- Validated: `make test`, `make race`, `make lint` all pass.
