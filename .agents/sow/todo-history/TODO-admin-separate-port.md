# TODO: Separate Admin Listener

## Purpose

Allow operators to expose the public website and public APIs on one listener,
while binding the admin UI and admin APIs to a different listener/port for
internal-only access over VPN or other trusted network paths.

Fit for purpose target:

- public traffic continues to flow through `cloudflare -> nginx -> app`
- admin traffic can be kept off the public listener and accessed via VPN /
  internal networking only
- the separation is explicit in the application, not only in reverse-proxy
  routing rules
- the runtime behavior is configurable enough to support:
  - local development with relaxed admin exposure
  - production with strict public/admin separation and authentication

## TL;DR

- The daemon currently has exactly one HTTP listener.
- Public routes and admin routes are mounted on the same mux and served by the
  same `http.Server`.
- The current product and spec require the admin surface to be access-controlled
  and fail-closed when admin auth is not configured.
- Adding a separate admin port and configurable dev/prod behavior is feasible,
  but it requires explicit decisions on admin exposure modes and on whether a
  no-auth development mode is allowed by contract.

## Current Status

- Implemented:
  - optional `--admin-listen`
  - explicit `--admin-auth-mode=required|disabled`
  - explicit `--allow-unauthenticated-admin`
  - split public/admin listeners with public-side `404` for admin routes
  - runtime `public_base_url` config for admin-to-public links
  - admin status payload includes `public_base_url`
  - systemd unit defaults that can be overridden via drop-in environment-backed
    listener/auth args
- Verified:
  - targeted Go tests for split listeners, unsafe auth-mode validation, and
    unauthenticated dev mode
  - full `go test ./...`
  - `pnpm --dir ui build`
  - `pnpm --dir ui lint`
- Found during install verification:
  - the systemd unit uses optional environment-backed CLI slots
  - when an optional slot expands to an empty string, the daemon receives an
    empty argv entry
  - Go's `flag` parser stops at the first non-flag argument, so later flags
    such as `--admin-auth-mode=disabled` are ignored
  - evidence:
    - live process argv from `/proc/<pid>/cmdline` showed:
      - `--listen`
      - `:18888`
      - `""`
      - `--admin-auth-mode=disabled`
      - `--allow-unauthenticated-admin`
    - admin endpoint still returned `503 admin authentication is not configured`
  - implication:
    - the install/systemd override path is not yet fully correct until the
      daemon strips empty argv entries before parsing flags

## Analysis

- Current CLI/runtime surface:
  - The daemon exposes only one listen flag today: `--listen`.
  - Evidence:
    - `cmd/update-ipsets/daemon.go:17-27`
    - `cmd/update-ipsets/daemon.go:62-69`
- Current web server structure:
  - `pkg/web.Options` has a single `Listen` field.
  - `web.Run()` constructs one `http.Server` bound to `opts.Listen`.
  - Evidence:
    - `pkg/web/server.go:36-45`
    - `pkg/web/server.go:47-50`
    - `pkg/web/server.go:106-115`
    - `pkg/web/server.go:145-155`
- Current routing structure:
  - Public routes and admin routes are registered on the same `http.ServeMux`.
  - Admin HTML routes:
    - `/admin`
    - `/admin/*`
  - Admin APIs:
    - `/api/v1/admin/*`
  - Evidence:
    - public route registration starts at `pkg/web/server.go:163-220`
    - admin route registration is at `pkg/web/server.go:404-418`
- Current install/deploy defaults:
  - The shipped systemd unit uses one listener: `--listen :18888`.
  - Evidence:
    - `install.sh:197`
- Current spec position:
  - The admin surface must be separate from the public site, but the specs do
    not currently require a separate listener/port.
  - Evidence:
    - `specs/admin-ui.md:31-53`
    - `specs/website.md:72-95`
- Current auth behavior:
  - Admin HTML and admin APIs are both wrapped by `basicAuth`.
  - If `UPDATE_IPSETS_ADMIN_USER` or `UPDATE_IPSETS_ADMIN_PASSWORD` is unset,
    the handler returns `503 admin authentication is not configured`.
  - There is no current "no-auth dev mode".
  - Evidence:
    - `pkg/web/server.go:404-418`
    - `pkg/web/middleware.go:87-103`
- New user requirement:
  - The behavior must become configurable enough to support:
    - development: same port, no auth
    - production: separate admin port, with auth
  - This is a real product/security contract change, not just a small
    implementation detail, because "no auth" contradicts the current normative
    admin contract.
- User correction on prior safety proposal:
  - A loopback-only rule is not a valid safety proxy for unauthenticated admin.
    In real deployments, nginx or another local reverse proxy may connect from
    localhost while still exposing the admin surface to the internet.
  - Therefore the app cannot safely infer "private" from the bind address
    alone; any unauthenticated-admin mode must be an explicit operator choice,
    not a heuristic based on loopback binding.

## Decisions

- User-approved decisions:
  - `1. A` Support explicit configurable knobs for listener split and auth mode.
  - `2. A` Add optional `--admin-listen`.
  - `3. A` When `--admin-listen` is set, remove admin routes from the public
    listener and return `404`.
  - `4. B` Allow unauthenticated admin only as an explicit dangerous mode,
    requiring a second acknowledgment knob.
  - `5. A` Use explicit auth-mode configuration rather than inferring from
    missing credentials.
  - `6. A` Keep `/healthz` on the public/shared listener only.
  - `7. A` Define a dedicated public website base URL in config and use it for
    admin-to-public links when admin runs on a different listener.
  - `8. A` For user's local installation, run admin on the shared public
    listener and disable admin auth explicitly via the unsafe acknowledgment
    knob.

### 8. Local installation operating mode

Context:

- The implementation is now configurable.
- user wants this workstation installation to run in development mode:
  - single listener
  - admin on the same port
  - admin auth disabled

Selected option:

- A. Configure the installed systemd service with:
  - no `--admin-listen`
  - `--admin-auth-mode=disabled`
  - `--allow-unauthenticated-admin`

Implications:

- `/admin` and `/api/v1/admin/*` stay on `:18888`
- no Basic Auth challenge is enforced on this installation
- this is intentionally unsafe for internet-exposed deployments and is only
  acceptable because this installation is for local development

### 1. Exposure model must be configurable

Context:

- The user explicitly wants different operational modes:
  - development: same port, no auth
  - production: different port, auth
- Today the app supports only one mode:
  - same port
  - admin always access-controlled
  - fail-closed when auth vars are missing

Options:

- A. Support multiple explicit admin exposure modes in the application. Recommended.
  - Example modes:
    - shared listener + auth required
    - split listener + auth required
    - shared listener + auth disabled
    - split listener + auth disabled
  - Pros:
    - matches the requested flexibility directly
    - no hidden behavior; operator chooses the mode explicitly
  - Implications:
    - new CLI/config surface
    - specs must define which combinations are valid
  - Risks:
    - dangerous combinations become possible unless guarded carefully

- B. Support only two opinionated modes:
  - dev mode
  - production mode
  - Pros:
    - simpler user experience
    - fewer invalid combinations
  - Implications:
    - less composable
    - product logic becomes mode-driven rather than capability-driven
  - Risks:
    - harder to extend later
    - less transparent than explicit listener/auth settings

Recommendation:

- `1. A`

### 2. Listener model

Context:

- Today the app has one listener and one mux.
- The configurable exposure model above still needs a concrete listener model.

Options:

- A. Add an optional `--admin-listen` listener and keep `--listen` for public traffic only when `--admin-listen` is set. Recommended.
  - Pros:
    - clean public/admin separation in the app
    - public port no longer exposes `/admin` or `/api/v1/admin/*`
    - aligns best with your VPN-only admin use case
  - Implications:
    - two muxes or one mux with route filtering
    - install/systemd/docs/specs/tests all need updates
  - Risks:
    - compatibility change for operators who currently expect admin on the public port when `--admin-listen` is configured

- B. Add an optional `--admin-listen` listener but keep admin routes on the public listener too.
  - Pros:
    - lower compatibility risk
    - easiest transition for existing installs
  - Implications:
    - admin exists on two ports at the same time
  - Risks:
    - weak security model
    - easier operator confusion
    - does not fully solve accidental public admin exposure

- C. Do not change the app; use nginx only.
  - Pros:
    - no code change
    - fastest operational workaround
  - Implications:
    - app remains single-listener internally
    - separation depends entirely on proxy configuration
  - Risks:
    - less robust than application-level separation
    - easier for future deployments to regress if proxy rules drift

Recommendation:

- `2. A`

### 3. Compatibility behavior when `--admin-listen` is configured

Context:

- If we add a second listener, we need to decide what happens to admin routes on
  the public listener.

Options:

- A. When `--admin-listen` is set, remove admin routes from the public listener completely and return `404` there. Recommended.
  - Pros:
    - strongest separation
    - unambiguous behavior
    - safest default for your deployment model
  - Implications:
    - some old bookmarks/scripts hitting the public port for admin will break after opt-in
  - Risks:
    - transition work for operators

- B. When `--admin-listen` is set, leave admin routes on public but redirect `/admin` to the admin port.
  - Pros:
    - friendlier transition
  - Implications:
    - redirect target must be configured correctly
    - admin APIs still need a clear behavior
  - Risks:
    - easy to misconfigure
    - still leaks admin surface intent on the public listener

- C. When `--admin-listen` is set, keep returning `403` on public admin routes.
  - Pros:
    - explicit denial
  - Implications:
    - extra route-specific branching
  - Risks:
    - not materially better than `404` for your use case

Recommendation:

- `3. A`

### 4. Authentication policy must be configurable

Context:

- The user explicitly wants "no auth" in development.
- The current spec says the admin surface MUST be access-controlled and MUST
  fail closed if auth is not configured.
- So allowing no-auth is not just an implementation choice; it is a deliberate
  contract change.
- The app cannot reliably infer a safe no-auth environment from the bind
  address, because localhost may sit behind a public reverse proxy.

Options:

- A. Keep Basic Auth mandatory in all modes.
  - Pros:
    - preserves current security contract
    - simplest security story
  - Implications:
    - does not satisfy the requested dev workflow
  - Risks:
    - friction in development remains

- B. Add an explicit no-auth mode on any topology, but require a second explicit "dangerous" acknowledgment knob to enable it. Recommended.
  - Example:
    - `--admin-auth-mode=disabled`
    - plus `--allow-unauthenticated-admin`
  - Pros:
    - satisfies the dev requirement
    - does not rely on false safety heuristics
    - makes the dangerous choice unmistakably deliberate
    - works for localhost, VPN-only, or proxy-mediated topologies equally
  - Implications:
    - startup validation must reject `disabled` unless the dangerous
      acknowledgment is also present
    - logs/docs/specs must call this mode unsafe and operator-managed
  - Risks:
    - still a dangerous mode if selected carelessly

- C. Add an unrestricted no-auth mode with no extra acknowledgment.
  - Pros:
    - simplest UX
  - Implications:
    - one flag flips admin to unauthenticated
  - Risks:
    - too easy to misconfigure
    - directly weakens the current safety posture more than necessary

Recommendation:

- `4. B`

### 5. Configuration surface for auth

Context:

- If auth becomes configurable, we need a concrete user-facing interface.

Options:

- A. Add an explicit flag/config enum such as `--admin-auth-mode=required|disabled`. Recommended.
  - Pros:
    - explicit and understandable
    - easy to validate against bind addresses
  - Implications:
    - new CLI/docs/spec surface
  - Risks:
    - none beyond the need to document it clearly

- B. Infer auth mode implicitly from whether env vars are set.
  - Pros:
    - fewer knobs
  - Implications:
    - ambiguous behavior
    - easy to disable auth accidentally
  - Risks:
    - too error-prone for production

Recommendation:

- `5. A`

### 6. What to do with health/status endpoints

Context:

- `/healthz` is currently on the single listener.
- We need to decide whether the admin listener should also expose a health check.

Options:

- A. Keep `/healthz` on the public listener only; admin listener serves only admin HTML and `/api/v1/admin/*`. Recommended.
  - Pros:
    - keeps responsibilities clean
    - avoids duplicate surface
  - Implications:
    - internal admin-only monitoring would need either public health over internal path or a future dedicated admin health endpoint
  - Risks:
    - some operators may want admin-port liveness checks later

- B. Expose `/healthz` on both listeners.
  - Pros:
    - operational convenience
  - Implications:
    - not purely admin-only anymore
  - Risks:
    - duplicated semantics; small surface expansion

Recommendation:

- `6. A`

### 7. Public website base URL for admin links

Context:

- The admin UI currently includes relative links to public website pages such as
  `/ipsets/<name>`.
- With true listener separation, those relative links would resolve against the
  admin listener origin and hit `404`, because public pages will no longer exist
  on the admin listener.
- There is already a runtime `web_url` config value, but it is currently the
  published ipset page prefix used by engine-generated metadata and sitemap
  output, not the generic website origin/base URL.
- Evidence:
  - admin UI relative public links:
    - `ui/src/components/admin/feed-modal.tsx:250`
    - `ui/src/components/admin/feeds-table.tsx:95`
  - existing `web_url` runtime field and default:
    - `pkg/config/config.go:117`
    - `pkg/config/config.go:483`
  - existing `web_url` usage appends feed names to it:
    - `pkg/engine/output.go:722-735`
    - `pkg/engine/metadata.go:218-237`

Options:

- A. Add a separate config field for the public website base URL and use that
  from the admin UI for links to public pages. Recommended and approved.
  - Pros:
    - preserves the current meaning of `web_url`
    - keeps routing explicit in split-port deployments
    - avoids breaking engine metadata/sitemap link generation
  - Implications:
    - config/spec/docs/install/test updates
    - admin SPA needs a runtime/public config value
  - Risks:
    - one more config knob to document

- B. Reuse `web_url` as the public website base URL.
  - Pros:
    - no new config key
  - Implications:
    - changes the current meaning of `web_url`
    - may require reworking engine-generated links and defaults
  - Risks:
    - compatibility risk
    - easy confusion between website origin and published feed-page prefix

Recommendation:

- `7. A`

## Plan

1. Completed: the seven design decisions above were confirmed with the user.
2. Completed: specs now define dual-listener behavior, auth modes, safety
   rules, and the new public website base URL.
3. Completed: daemon/web options now expose an optional admin listener,
   explicit auth mode, and a dedicated public website base URL from config.
4. Completed: startup validation now rejects unsafe no-auth combinations and
   split-listener startup without `runtime.public_base_url`.
5. Completed: route registration is split into public-only and admin-only
   handlers.
6. Completed: the admin UI consumes the configured public website base URL and
   no longer uses same-origin relative public links.
7. Completed: install/systemd/docs were updated for the new listener/auth
   configuration.
8. Completed: tests were added for split listeners, auth validation, dev
   no-auth mode, and admin status exposure of `public_base_url`.
9. Completed: targeted tests and broader verification passed.

## Implied Decisions

- This should be an opt-in feature so current single-port installs keep working
  unchanged when `--admin-listen` is not set.
- The public listener should continue to serve the current public website and
  public APIs unchanged.
- The admin listener should host only the operator surface, not a second copy of
  the public site.
- If a no-auth mode is introduced, it should be explicitly selected, never
  inferred accidentally from missing credentials or bind-address heuristics.
- Admin UI links that point to public website pages should be built from the
  explicit configured public website base URL, not from same-origin relative
  paths.

## Testing Requirements

- Unit/integration tests for mux separation and route exposure.
- Tests for auth behavior on both listeners.
- Tests for startup and graceful shutdown with two servers.
- Tests for admin UI/public link generation using the configured public website
  base URL.
- Verification of install/systemd arguments and default compatibility.

## Verification Results

- Added backend coverage in `pkg/web/feature_test.go` for:
  - split public/admin listeners
  - `runtime.public_base_url` enforcement in split mode
  - explicit unauthenticated-admin acknowledgement enforcement
  - unauthenticated admin dev mode
- Passed:
  - `go test ./pkg/web ./pkg/config ./pkg/engine ./cmd/update-ipsets`
  - `go test ./...`
  - `pnpm --dir ui build`
  - `pnpm --dir ui lint`
  - `bash -n install.sh`

## Documentation Updates Completed

- `specs/admin-ui.md`
- `specs/config.md`
- `specs/website.md`
- `README.md`
- `install.sh` comments / service summary text
