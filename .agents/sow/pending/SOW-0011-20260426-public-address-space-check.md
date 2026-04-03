# SOW-0011 | 2026-04-26 | public-address-space-check

## Status

open
public page, API docs, telemetry, and MCP integration scope clarified by Costa;
accepted formats, limits, privacy, and abuse controls still need design

## Requirements

Given users can already check a single IP, when this SOW is complete, then users must be able to check their own IP address space for feed matches using a text area and file upload.

Given user-provided address spaces can be large or malformed, when inputs are accepted, then accepted formats, size limits, validation errors, cancellation, rate limits, and privacy semantics must be explicit.

Given this is a public compute surface, when matching runs, then CPU, memory, I/O, and response latency must be bounded and observable.

Given this feature is part of the public website and public machine interface,
when this SOW is complete, then it must include a dedicated public page,
documented APIs, telemetry data for operator visibility, and corresponding MCP
tools.

## Analysis

Initial sources to consult:

- Existing single-IP query/search flows.
- Existing `pkg/iprange` parsers and set operations.
- Existing public API rate limiting.
- `.agents/sow/specs/website.md`, `.agents/sow/specs/operating-principles.md`, and `.agents/sow/specs/memory-management.md`.

Current known context:

- The old tracker called this "paste an `{ip,net}set` and tell overlaps".
- This should be similar in user intent to single-IP lookup, but it is much heavier.
- 2026-04-28 Costa decision: SOW-0010 and SOW-0011 each need their own public
  page, documented APIs to use, telemetry data, and MCP tools.

## Implications and decisions

- File upload introduces input validation and potential abuse risks.
- Results may need to be ephemeral; bookmarkable results require storage and retention decisions.
- This should not bypass existing cheap-serving principles.
- Upload/paste matching is a heavier compute surface than browsing generated
  artifacts; telemetry, cancellation, and rate limiting are part of the product
  contract, not implementation details.
- MCP support should reuse the same backend contract as the public page and API;
  coordinate with `SOW-0013`.

## Plan

Chunked SOW - reasoning: parsing, backend matching, UX, and abuse controls are separate risks.

1. `input-contract` - high risk
2. `matching-design` - high risk
3. `public-ui` - medium risk
4. `rate-limit-and-observability` - high risk
5. `api-and-mcp-contract` - high risk
6. `docs-and-tests` - medium risk

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
