# Classification rules (shared across iplists.firehol.org agents)

These rules govern how the agent assigns `license`, `redistributable`,
`redistribution_notes`, and related classification fields. The rules apply
to every enrichment and evaluation agent that includes this file.

## License and redistribution — direct-upstream-only scope

`license`, `redistributable`, and `redistribution_notes` reflect ONLY the
terms of THIS feed's **direct upstream** — the URL the catalog actually
downloads from. They do NOT reflect terms found anywhere further up the
supply chain (e.g., if the direct upstream is a GitHub mirror that
republishes another organization's data, the original organization's
terms are out of scope for these fields).

You MAY surface upstream-of-upstream license or redistribution
information in `research_notes` (free-form markdown) for operator
context. You MUST NOT use that information to set or alter
`redistributable`, `license`, or `redistribution_notes`.

Resolving the legal relationship between our direct upstream and its own
upstream is their concern, not ours.

### Defaults when the direct upstream is "publicly available"

"Publicly available" means: the URL responds to an **unauthenticated HTTP
GET** without requiring API keys, login, payment, or other special access.

- CDN-cached or rate-limited URLs that meet the above ARE publicly available.
- URLs behind login, paywall, or API token are NOT publicly available.

When the direct upstream IS publicly available:

- If no redistribution rule is stated by the direct upstream:
  `redistributable: true`
- If no license is stated by the direct upstream:
  `redistribution_notes.stated_license: "public feed"`

These defaults apply EVEN when restrictive terms are found at
upstream-of-upstream layers.

### What is NOT enough for `redistributable: false`

Set `redistributable: false` only when the direct upstream explicitly
forbids copying, redistribution, republication, mirroring, public
display, or public sharing of the feed data.

The following are NOT enough by themselves:

- non-commercial-only use;
- no-resale wording;
- all-rights-reserved wording;
- warranty disclaimers;
- attribution requirements;
- "use at your own risk" wording;
- API-rate or fair-use limits;
- unknown license with no explicit anti-redistribution language.

When terms restrict commercial use but do not explicitly forbid
redistribution, keep `redistributable: true`, record
`commercial_use_allowed: false` when supported by the schema, and explain
the restriction in redistribution terms.

### When the direct upstream is NOT publicly available

The defaults above do NOT apply. Classify from whatever access agreement
the catalog implies (e.g., the API ToS the catalog authenticates under).
If the catalog YAML already carries a `license:` string, treat it as
authoritative for the direct relationship unless your research finds an
explicit contradiction at the **direct** upstream.

## What this means in practice

### Concrete worked example — community mirror of a commercial provider

Suppose the catalog YAML says:
```
url: https://raw.githubusercontent.com/exampleuser/example-mirror/main/list.ipv4
maintainer: ExampleVendor / exampleuser
```
And your research finds:
- the GitHub repo `exampleuser/example-mirror` has NO LICENSE file, no
  README terms about redistribution, and is publicly cloneable;
- `ExampleVendor`'s own corporate website carries a Terms of Service
  prohibiting redistribution of their data.

The direct upstream is the **GitHub raw URL**. It is publicly available
(unauthenticated HTTP GET works) and states no rule. The ExampleVendor
ToS is at an **upstream-of-upstream** layer.

CORRECT enrichment output:

- `redistribution_notes.stated_license`: `"public feed"`
- `redistribution_notes.redistribution_terms`: `null`
- `redistribution_notes.attribution_required`: `null`
- `redistribution_notes.commercial_use_allowed`: `null` (or `true` if
  publicly downloadable with no terms is treated as permitting; default
  to `null` when uncertain)
- `redistribution_notes.evidence[0].source_url`:
  `https://github.com/exampleuser/example-mirror` (the DIRECT upstream's
  project page or raw URL) — NOT `https://www.examplevendor.com/legal`.
- `research_notes` MAY include:
  ```
  { "kind": "license_uncertainty",
    "description": "ExampleVendor's own ToS prohibits redistribution of their
    data. Our direct upstream is exampleuser's GitHub mirror, which has no
    LICENSE and is publicly downloadable; per the direct-upstream-only rule,
    that mirror's silence governs our classification. Operators redistributing
    further should consider the upstream-of-upstream context independently.",
    "evidence": [ { "source_url": "https://www.examplevendor.com/legal" } ] }
  ```

INCORRECT outputs (rule violations — DO NOT produce these):

- `stated_license: "ExampleVendor Terms of Service"` — that's the
  upstream-of-upstream's license; the direct upstream (the GitHub mirror)
  did not state it.
- `commercial_use_allowed: false` with evidence pointing at
  `examplevendor.com/legal` — same violation.
- `redistribution_terms: "Data may be used for personal use only..."` —
  copying ExampleVendor's wording into our direct-relationship field.

### Other shapes — quick patterns

- **Direct upstream is the original publisher's own website** with a
  stated license like `CC BY-NC-ND 3.0 (modified)` → use that license
  verbatim. The publicly-available defaults do not apply because a
  license is stated.

- **Direct upstream is a paid commercial API** the catalog authenticates
  against → upstream is NOT publicly available; defaults do not apply;
  classify from the API agreement terms.

- **Direct upstream republishes data behind its OWN notice** like "do not
  use commercially" or "do not resell" → record the use/commercial
  restriction in `redistribution_terms` and `commercial_use_allowed`; do
  NOT set `redistributable: false` unless the same direct upstream also
  explicitly forbids redistribution, republication, copying, mirroring,
  public display, or public sharing.
