# SOW-0087 | 2026-05-16 | markdown-disabled-admin-visibility

## Status

open

## Requirements

### Purpose

Make markdown generation availability visible to operators when the subsystem is
disabled or misconfigured, so a missing template directory cannot silently leave
MCP `fetch_analysis` without current markdown artifacts.

### User Request

Follow-up from SOW-0085. The user did not explicitly request this work in the
current turn; it tracks the remaining valid operational visibility gap found
while closing the markdown artifact regression.

### Assistant Understanding

Facts:

- SOW-0085 found that missing markdown templates only produced DEBUG-level logs.
- The project requires background work to be visible through the admin API/UI.
- Markdown artifacts are generated in background processing, not on public
  request paths.

Required outcome:

- Operators can see whether markdown generation is enabled, disabled because no
  templates are loaded, or failing during generation.
- Any admin UI/API change must be backed by source status, not by parsing logs.

### Acceptance Criteria

- Admin API exposes markdown generation status with enough detail to diagnose
  missing templates.
- Admin UI surfaces the disabled/error state without requiring log access.
- Logs use an appropriate severity when markdown generation is expected but
  templates cannot be loaded.
- Specs and operator docs are updated if the admin API/UI contract changes.
- Validation covers the disabled-template and enabled-template paths.

## Pre-Implementation Gate

Not filled yet. This SOW is a pending follow-up only; the gate must be completed
before implementation starts.

## Notes

This SOW is intentionally separate from SOW-0085 because SOW-0085's accepted
scope was artifact generation and MCP markdown content. Admin visibility is a
related operational improvement, but it changes admin status/UI contracts and
deserves focused analysis and validation.
