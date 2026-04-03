# AI Classification Rules — Specification

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY**
describe required behavior of the product and of every AI agent the
product invokes.

## Purpose

This document defines the scope and defaults for `license`,
`redistributable`, `redistribution_notes`, and related classification
fields — in AI-generated enrichment output, in evaluation output, and in
the underlying catalog YAML.

The product cannot audit transitive supply chains for hundreds of feeds.
This document encodes a pragmatic, consistent rule that scopes our legal
exposure to direct upstream relationships only.

## Scope

These rules apply to:

- The feed enrichment agent (`agents/feed-enrichment.ai`) and its outputs
- The feed evaluation agent (when implemented) and its outputs
- Any future AI agent that classifies feed redistribution or licensing
- The engine's interpretation of `license` and `redistributable` YAML
  fields in `configs/firehol/sources/*/<feed>.yaml`
- The public website rendering of license and redistribution information

The rules do NOT govern factual `research_notes` content — agents MAY
record upstream-of-upstream observations there as operator context.

## License and redistribution — direct-upstream-only scope

A feed's `license`, `redistributable`, and `redistribution_notes` fields
MUST reflect only the terms of the feed's **direct upstream** — the URL
the catalog downloads from.

These fields MUST NOT be set or altered based on terms found at
upstream-of-upstream layers (e.g., if the direct upstream is a GitHub
mirror that republishes another organization's data, the original
organization's terms do not affect our flags).

Agents MAY record upstream-of-upstream license or redistribution terms in
`research_notes` with `kind: license_uncertainty` for operator context.
Agents MUST NOT use that information to change the classification fields.

Resolving the legal relationship between our direct upstream and its own
upstream is the responsibility of the involved parties, not the product.

## Definition: "publicly available"

A direct upstream URL is **publicly available** if and only if the URL
responds to an **unauthenticated HTTP GET** without requiring API keys,
login, payment, or other special access.

- CDN-cached or rate-limited URLs that meet the above ARE publicly available.
- URLs behind login, paywall, or API token are NOT publicly available.

## Defaults for publicly available upstreams

When the direct upstream is publicly available AND the catalog YAML or
direct-upstream research evidence does not state a redistribution rule:

- `redistributable` MUST default to `true`.

When the direct upstream is publicly available AND the catalog YAML or
direct-upstream research evidence does not state a license:

- `license` MUST default to the string `"public feed"`.
- `redistribution_notes.stated_license` MUST mirror this default.

These defaults MUST apply even when restrictive terms are found at
upstream-of-upstream layers.

## What is not enough for `redistributable: false`

A source MUST NOT be marked non-redistributable merely because the direct
upstream has one of these terms:

- non-commercial-only use;
- no-resale wording;
- all-rights-reserved wording;
- warranty disclaimers;
- attribution requirements;
- "use at your own risk" wording;
- API-rate or fair-use limits;
- unknown license with no explicit anti-redistribution language.

`redistributable: false` is allowed only when the direct upstream
explicitly forbids copying, redistribution, republication, mirroring, or
public display, or public sharing of the feed data.

When direct-upstream terms restrict commercial use but do not explicitly
forbid redistribution, agents MUST keep `redistributable: true`, record
the commercial-use restriction in the applicable redistribution terms, and
set `commercial_use_allowed: false` where the schema supports it.

## When the direct upstream is NOT publicly available

The defaults above MUST NOT apply. Classification fields MUST be set
from whatever terms the direct upstream's access agreement provides
(e.g., the API ToS the catalog authenticates under).

If the catalog YAML already carries a `license:` string for a
non-publicly-available upstream, that string SHOULD be treated as
authoritative for the direct relationship unless research finds an
explicit contradiction at the **direct** upstream (not at an
upstream-of-upstream).

## Audit trail

All redistribution/license decisions MUST be reproducible from the
catalog YAML plus the enrichment file alone. The engine MUST NOT silently
override these values at runtime.

## Cross-references

- Operational form for agents: `agents/shared/classification-rules.md`
  (rendered into agent prompts via `{% render %}`).
- AGENTS.md "Source-of-source legal scope" section quotes this spec.
- Public end-user methodology page:
  `pkg/web/static/methodology/ai-research-license-rules.md`.
- The catalog YAML field reference in
  `.agents/sow/specs/feeds.md` "Legal/publication policy" section defers
  to this spec for classification scope.

## Compatibility with existing catalog YAML

Catalog entries that pre-date this spec MAY have `license` strings that
were set under different rules. The product is NOT required to retroactively
re-evaluate them. New enrichment runs apply this spec; older recorded
values stand until an enrichment run produces a high-confidence
`catalog_corrections` entry that updates them.
