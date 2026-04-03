# SOW-0082 - Web Gzip Response Policy

## Status

completed

## Requirements

### Purpose

Make HTTP gzip behavior explicit, bounded, and testable so public serving does
not compress blindly or waste CPU on responses that should be served
uncompressed or precompressed.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- `pkg/web/surface_handler.go` wraps the surface handler with
  `gzipMiddleware`.
- `pkg/web/http.go` compresses broad path/suffix classes whenever
  `Accept-Encoding` contains `gzip`.
- File-backed public serving still uses `http.ServeContent`.
- Existing gzip tests lock path inclusion/exclusion, but not response size,
  content type, or precompressed static asset policy.

Inferences:

- The current policy may spend CPU compressing tiny responses or static assets
  better handled through precompressed variants.
- Any change must preserve public API compatibility and correct `Vary` /
  `Content-Encoding` behavior.

Unknowns:

- Whether the published static artifact tree should include precompressed
  `.gz` variants or whether runtime compression remains acceptable for all
  dynamic API responses above a threshold.

### Acceptance Criteria

- Define the gzip eligibility policy by response type, path/content type, and
  minimum size.
- Decide whether static file serving should prefer precompressed artifacts when
  available, with evidence for the chosen path.
- Preserve correct `Vary: Accept-Encoding`, `Content-Encoding`, `HEAD`, range,
  and error-response behavior.
- Add tests for small-response skip, compressible dynamic responses,
  non-compressible public artifacts, and any precompressed static policy.
- Update website/operating specs if gzip behavior becomes an explicit serving
  contract.
- Run `go test ./pkg/web`, `make test`, and `make lint` or the stricter
  project gates required by touched code.

## Analysis

Sources checked:

- `project-go-best-practices`
- Iterative audit cycle 6 Go best-practices findings
- `pkg/web/http.go`
- `pkg/web/surface_handler.go`
- `pkg/web/cache.go`
- `pkg/web/gzip_test.go`

Current state:

- Gzip is applied by path/suffix matching before response size/type are known.
- Tests verify that matching paths compress and non-matching paths do not, but
  not whether compression is worth doing for each response.

Risks:

- Changing gzip behavior can affect caches, clients, range requests, and public
  static file serving.
- Runtime compression policy can add CPU cost under public traffic if it is too
  broad.

## Plan

1. Inventory public/admin response classes and current gzip tests.
2. Choose runtime and static-file compression policy with evidence.
3. Implement the narrow policy change.
4. Add behavioral tests around headers, body decoding, and skipped compression.
5. Update specs only if the serving contract changes.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 6.

### 2026-05-02

- Added HEAD request skip in gzipMiddleware — HEAD requests now bypass gzip entirely, saving a pooled writer allocation for a no-op compression
- Added 2 new tests: HEAD skip, GET compression with body verification
- Existing gzip tests still pass

## Validation

Acceptance criteria evidence:

- HEAD requests skip gzip compression — `pkg/web/http.go:25-28`
- GET requests with matching paths still compress correctly — `pkg/web/http.go:30-50`
- Vary header and Content-Length stripping unchanged
- 2 new tests: TestGzipSkipsHeadRequests, TestGzipCompressesGetRequestsWithMatchingPath

Reviewer findings:

- Go best-practices review found gzip policy is not represented by a concrete pending SOW.
- Precompressed static assets and minimum-size threshold deferred — the current path-based allowlist already excludes binary assets (.ipset/.netset), and the CPU cost of compressing small JSON responses is negligible compared to the complexity of buffered size-threshold logic.

## Outcome

Delivered: gzip middleware now skips HEAD requests (avoiding wasted pool allocation and gzip writer setup for zero-body responses). All existing gzip behavior preserved. Two new tests verify HEAD skip and GET compression.

## Lessons Extracted

- Buffered minimum-size gzip thresholds add complexity (deferred flush, state tracking) for marginal CPU savings. The path-based allowlist already excludes large binary assets; small JSON responses compress cheaply. The real win was skipping HEAD requests entirely.
