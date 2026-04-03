# SOW-0010 | 2026-04-26 | public-feed-composer

## Status

open
public page, API docs, telemetry, and MCP integration scope clarified by Costa;
UX, limits, and cache/rate behavior still need approval before implementation

## Requirements

Given users need custom blocklists, when this SOW is complete, then the public site must provide a page to build a custom feed by including and excluding feeds/IPs.

Given composing feeds can be expensive, when users run compositions, then request size, execution time, cancellation, rate limits, output formats, and cacheability must be bounded.

Given backend compose groundwork exists, when the public UI is added, then it must reuse existing API semantics where fit and define any missing contract explicitly.

Given this is a public product capability, when this SOW is complete, then it
must include a dedicated public page, documented APIs, telemetry data for
operator visibility, and corresponding MCP tools.

## Analysis

Initial sources to consult:

- Existing `/api/v1/compose` backend route.
- README public API table.
- `.agents/sow/specs/website.md`, `.agents/sow/specs/homepage.md`, and `.agents/sow/specs/operating-principles.md`.
- Existing feed explorer/search UI patterns.

Current known context:

- The old tracker says backend compose groundwork exists, but public UI/release contract is incomplete.
- 2026-04-28 Costa decision: SOW-0010 and SOW-0011 each need their own public
  page, documented APIs to use, telemetry data, and MCP tools.

## Implications and decisions

- Public composer can create server load and must not become an unbounded compute endpoint.
- UX must make include/exclude semantics obvious.
- Rate limits and output formats must be documented.
- Telemetry must be designed with the backend execution path; adding UI without
  operator-visible CPU/memory/I/O/request behavior would miss the purpose.
- MCP support should be a first-class contract, not an afterthought; coordinate
  with `SOW-0013`.

## Plan

Chunked SOW - reasoning: API contract, frontend, and load controls are separable.

1. `contract-and-limits` - high risk
2. `frontend-ux` - medium risk
3. `backend-adjustments-and-telemetry` - high risk
4. `api-and-mcp-contract` - high risk
5. `docs-and-tests` - medium risk

## Execution log

Pending.

## Validation

- [ ] Acceptance criteria evidence
- [ ] Real-use validation evidence
- [ ] Cross-model reviewer findings (logged + addressed)
- [ ] Lessons extracted (or "none, reasoning: ...")
- [ ] Same-failure-at-other-scales check

## Outcome

Pending.

## Lessons extracted

Pending.
