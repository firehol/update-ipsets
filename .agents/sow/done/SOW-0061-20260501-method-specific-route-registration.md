# SOW-0061 - Method-Specific Route Registration

## Status

Status: completed

Sub-state: implemented and validated

## Requirements

### Purpose

Make HTTP route contracts explicit so read handlers do not accept write methods and unsupported methods return correct `405 Method Not Allowed` responses.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- Public API routes use bare `mux.HandleFunc("/path", ...)` registration at `pkg/web/routes.go:15` through `pkg/web/routes.go:44`.
- Most public handlers do not check `r.Method`.
- Selected admin/integrity handlers manually check methods in places such as `pkg/web/routes.go:293`, `pkg/web/admin.go:320`, and `pkg/web/integrity.go:95`.
- The website spec describes endpoint families by HTTP method.
- Official Go 1.22+ `net/http.ServeMux` supports method patterns and automatic `405` handling: https://go.dev/doc/go1.22

Inferences:

- `POST` requests can reach some read handlers today.
- Switching route registration may affect CORS/`OPTIONS` behavior and needs explicit tests.

Unknowns:

- Whether any legacy clients rely on non-GET methods against read endpoints.

### Acceptance Criteria

- Public and admin routes are registered with explicit method patterns where supported.
- Unsupported methods return `405` with appropriate `Allow` behavior, except where CORS `OPTIONS` is intentionally handled.
- Existing admin write routes remain `POST` only.
- Tests cover representative public GET, admin POST, unsupported method, and CORS preflight behavior.
- Specs/docs are updated if any method compatibility changes are intentional.

## Analysis

Sources checked:

- `pkg/web/routes.go`
- `pkg/web/admin.go`
- `pkg/web/integrity.go`
- `.agents/sow/specs/website.md`
- Go 1.22 release notes for `ServeMux` method patterns.

Current state:

- Route method constraints are mostly implicit or manual.

Risks:

- Dynamic read endpoints can do work for unexpected methods.
- A method-registration migration can accidentally break prefix routes or CORS if not tested carefully.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Route migration strategy
   - A. Migrate all routes in one pass with comprehensive tests.
     - Pros: consistent final state.
     - Cons: higher blast radius.
   - B. Migrate by surface: public API, admin API, static/direct artifacts. Recommended.
     - Pros: easier review and preserves CORS behavior deliberately.
     - Cons: temporary mixed style.
   - C. Keep current routes and add method checks in each handler.
     - Pros: less router churn.
     - Cons: duplicates stdlib behavior and is easier to miss.

## Plan

1. Inventory routes, expected methods, and CORS behavior.
2. Convert route registration to method patterns by surface.
3. Remove redundant method checks where stdlib routing owns the behavior.
4. Add real HTTP server tests for 405 and `OPTIONS`.
5. Update specs and project skills with route-method expectations.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved from pending to current for autonomous implementation cleanup.
- Converted public API, admin API, embedded asset, public artifact, and SPA
  route registrations in `pkg/web/routes.go` to Go `ServeMux` method patterns.
- Registered mixed admin prefix routers for both GET and POST while preserving
  action-level method checks where the suffix determines read versus mutation.
- Added explicit 405 handlers for POST-only admin routes that otherwise would
  be shadowed by the GET SPA fallback.
- Preserved legacy/unknown admin POST paths as 404 where they are not supported
  routes.
- Added real HTTP route-method tests covering representative public GET,
  public unsupported POST, admin POST, admin unsupported GET, feed action
  unsupported GET, and CORS preflight behavior.
- Updated website/admin specs and the project coding skill with route-method
  expectations.

## Validation

Acceptance criteria evidence:

- Public routes in `pkg/web/routes.go` are registered with `GET` method
  patterns.
- Admin read routes are registered with `GET` method patterns; admin action
  routes are registered with `POST` method patterns.
- Mixed admin action prefixes for feed/artifact actions are registered for both
  GET and POST so suffix-level handlers can return action-specific 405s.
- `rg -n 'mux\\.Handle(Func)?\\("/' pkg/web/routes.go` returned no bare
  methodless route registrations.
- `TestRouteMethodContracts` covers public read success, public wrong-method
  405 with `Allow`, admin action POST success, admin wrong-method 405 with
  `Allow`, feed action wrong-method 405, and CORS `OPTIONS` 204 behavior.

Tests or equivalent validation:

- `go test ./pkg/web -run 'TestRouteMethodContracts|TestSurfaceHandlerModesRegisterExpectedSurfaces|TestAPIEndpointsAndCORS'` passed.
- `go test ./pkg/web` passed.

Real-use evidence:

- Pending.

Reviewer findings:

- Go best-practices review found method-unconstrained routes.

Same-failure scan:

- Scanned route registrations for bare methodless string patterns; none remain
  in `pkg/web/routes.go`.

Artifact maintenance gate:

- AGENTS.md: not needed; no project-wide workflow rule changed.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md`
  with the method-pattern routing rule.
- Specs: updated `.agents/sow/specs/website.md` and
  `.agents/sow/specs/admin-ui.md` with route-method contracts and 405
  behavior.
- End-user/operator docs: not needed; public/admin API specs already carry the
  durable method contract.
- End-user/operator skills: not needed; no exported operator skill changed.
- SOW lifecycle: moved from pending to current and then completed here.

Specs update:

- Updated `.agents/sow/specs/website.md` and
  `.agents/sow/specs/admin-ui.md`.

Project skills update:

- Updated `.agents/skills/project-coding/SKILL.md`.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Go `ServeMux` method patterns interact with broad GET SPA fallbacks: POST
  against unknown paths returns 405 through the fallback, while GET against
  POST-only exact API routes can fall through to the GET fallback unless an
  explicit 405 handler is registered.

Follow-up mapping:

- None.

## Outcome

Completed. HTTP route contracts are explicit at registration time, and
representative real-server tests verify 405 and CORS behavior.

## Lessons Extracted

Record method patterns together with broad SPA fallback behavior; exact
POST-only API routes may need explicit GET-side 405 handlers.

## Followup

None yet.
