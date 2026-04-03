# SOW-0018 | 2026-04-26 | content-header-footer

## Status

open
scope clarified by Costa: header and footer are primitive and need improvement;
exact design goals need discussion before implementation

## Requirements

Given methodology and content pages explain how the project works, when this SOW is complete, then all methodology pages and related public content must be reviewed, modernized, and updated.

Given public navigation affects trust and usability, when this SOW is complete, then header and footer content must be reviewed and improved.

Given the project should expose its source clearly, when this SOW is complete, then the public UI must include a GitHub icon/link in the appropriate header/footer location.

## Analysis

Initial sources to consult:

- `pkg/web/static/methodology/*.md`
- Public React header/footer components.
- README/specs that describe public pages.
- Existing design system and route structure.

Current known context:

- Several methodology pages exist and may lag current behavior.
- Header/footer review is part of release polish and public trust.
- 2026-04-28 Costa decision: the header and footer are primitive and need to be
  improved, but the desired direction needs discussion before implementation.

## Implications and decisions

- Content must match actual code behavior, not aspirational behavior.
- Header/footer should remain functional, not marketing-heavy.
- GitHub link target depends on upstream repo creation timing.
- Header/footer changes are public information architecture decisions. They
  should be designed with the site's real workflows, not treated as decorative
  polish.

## Plan

Chunked SOW - reasoning: content audit and UI shell changes can be reviewed separately.

1. `content-inventory` - medium risk
2. `methodology-updates` - medium risk
3. `header-footer-review` - medium risk
4. `github-link` - low risk
5. `validation` - medium risk

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
