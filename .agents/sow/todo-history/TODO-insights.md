# Insights — Deterministic Facts About Each Feed

## TL;DR

- A new `pkg/insights/` package that derives **factual observations** from each feed's processed data and publishes them as JSON.
- **All rules are deterministic and mathematical.** No statistical inference, no AI at runtime, no confidence labels in the UI. A rule either fires (publishes a fact) or stays silent.
- **Default percentile is p75**, not p50 (median) — IP lists are skewed and p75 captures "what most non-ephemeral IPs look like" better than the median.
- **All time-series rules use the last 500 recorded updates** as the window — never a fixed duration. The 500 is a constant from `RuntimeConfig.WebChartsEntries`.
- **15 starter rules** in this spec, all `Certain` (pure arithmetic). No LLM dependency. The brainstorm-with-LLMs step happens after Phase 3 ships, in a separate throwaway tool.
- **Output**: per-feed `<name>_insights.json` written by the engine. Frontend reads it via `/api/v1/sets/{name}/insights`.

## Decisions locked

| # | Decision | Choice |
|---|---|---|
| 1 | Package location | **`pkg/insights/`** — single package, all rules in one directory |
| 2 | Confidence labels in UI | **None** — fire or stay silent. No "Likely"/"Possibly" hedging. |
| 3 | LLM role | **None at runtime.** Brainstorm tool runs offline during dev only, after Phase 3 ships, output never published. |
| 4 | Default percentile | **p75** for age/duration headlines. p90 and p100 are secondary marks for multi-policy + permanent-bans rules. |
| 5 | Time-series window | **Last 500 recorded updates**, never a fixed time duration. The time span is a derived label, not the unit. |
| 6 | Where insights surface in UI | **(c) Both** — short list of `Certain` insights at top of feed page (orientation), in-context callouts beside the relevant chart |

## Architecture

```
pkg/insights/
├── insights.go              // public API: Derive(snap SignalSnapshot) []Insight
├── snapshot.go              // SignalSnapshot type — input shape
├── insight.go               // Insight type — output shape
├── rule.go                  // Rule type — function signature for rule functions
├── engine.go                // Engine.Run iterates rules, collects results
├── rules_size.go            // size variation, growth direction
├── rules_churn.go           // churn rate (fast/slow)
├── rules_age.go             // currently-listed-age + removed-age percentiles, multi-policy, permanent bans
├── rules_geography.go       // country concentration, country diversity, single-country
├── rules_asn.go             // ASN concentration, bogon contamination, infrastructure presence
├── rules_overlap.go         // subset, superset, derivative, independent, cross-category overlap
├── rules_freshness.go       // update lag, clock skew
├── insights_test.go         // engine-level tests
├── rule_*_test.go           // per-rule tests with synthetic snapshots covering boundary conditions
└── methodology_writer.go    // generates per-rule methodology page sections
```

**Why one package, not split into rule-categories**: every rule is small (~30 LoC), every rule is deterministic, every rule has its own test. Splitting would add ceremony without isolating any concern. If `pkg/insights` ever exceeds ~2000 LoC we can split. For 15-30 rules this stays small.

## Types

### Input: `SignalSnapshot`

The shape the engine assembles for each feed before invoking the insights engine. **Every field is already produced by the existing engine** — no new data sources needed.

```go
package insights

type SignalSnapshot struct {
    // Identification
    Name           string
    Category       string
    TrackedSinceTS int64    // unix seconds — feed's first observation by us
    SnapshotTS     int64    // unix seconds — when this snapshot was assembled

    // Size + churn series — last 500 RECORDED updates (not last 500 calendar days).
    // SizeSeries[i] is the unique IP count after the i-th recorded update.
    // ChurnSeries[i] = (added[i] + removed[i]) / size[i].
    // Both arrays have the same length (≤ 500). Empty for very young feeds.
    SizeSeries  []SizePoint
    ChurnSeries []ChurnPoint

    // Currently-listed IP age — histogram bucketed by hours.
    // AgeOfListed[h] = number of currently-listed IPs that have been listed for h hours.
    AgeOfListed AgeHistogram

    // Removed-IP duration — how long each removed IP was listed before being removed.
    // AgeOfRemoved[h] = number of IPs that were listed for h hours then removed.
    // This is the engine's `past` retention series.
    AgeOfRemoved AgeHistogram

    // Top facts from the comparison/attribution outputs.
    TotalIPs       uint64
    TopCountries   []CountryShare      // sorted desc by share
    TopASNs        []ASNShare          // sorted desc by share
    BogonShare     float64             // 0..1
    InfraShare     float64             // 0..1, from critical_infrastructure attribution
    Overlaps       []FeedOverlap       // pairwise overlaps with other feeds we track
    OverlapsByCat  map[string]float64  // category → max overlap with any feed in that category

    // Operational health (used by freshness rules only)
    LastUpdatedTS    int64   // unix seconds — last successful download
    ConfiguredFreqMin int    // configured frequency in minutes (0 = static)
    DownloadFailures int     // count from cache entry
    ClockSkewSeconds int64   // observed
}

type SizePoint struct {
    TS   int64
    Size uint64
}

type ChurnPoint struct {
    TS      int64
    Added   uint64
    Removed uint64
    Kept    uint64
    Size    uint64  // size after this update
}

type AgeHistogram struct {
    BucketsHours []int      // hour markers, sorted ascending
    Counts       []uint64   // IPs per bucket
    Total        uint64     // sum(Counts)
}

type CountryShare struct {
    Code  string  // ISO-3166 alpha-2
    Name  string  // country name
    IPs   uint64
    Share float64 // 0..1
}

type ASNShare struct {
    Number       uint32
    Name         string
    IPs          uint64
    Share        float64 // 0..1
    IsBogon      bool    // ASN is in bogon space (rare but possible)
    IsInfra      bool    // ASN is on the curated critical infrastructure list
}

type FeedOverlap struct {
    Other     string  // other feed name
    Category  string  // other feed's category
    OurShare  float64 // (this ∩ other) / this — fraction of THIS feed in the other
    TheirShare float64 // (this ∩ other) / other — fraction of OTHER feed in this
    OlderThanThis bool // is the other feed older than this one
}
```

### Output: `Insight`

```go
type Insight struct {
    Code       string         // stable identifier, never displayed; e.g. "country_concentrated"
    Section    Section        // which UI section the insight belongs in
    Headline   string         // user-facing one-liner — the ONLY text the UI shows
    Evidence   map[string]any // raw values that fired the rule (audit trail)
    Methodology string        // /methodology/<slug> link
}

type Section int
const (
    SectionOverview     Section = iota // appears in the top "What we noticed" callout
    SectionComposition                 // beside geo/asn/bogons
    SectionRetention                   // beside age charts
    SectionTrends                      // beside size/churn charts
    SectionRelationships               // beside overlaps
    SectionFreshness                   // beside operational health
)
```

**Critical**: there is NO `Confidence` field. Either a rule fires (publishes a fact) or it stays silent. The threshold IS the publish/no-publish decision. There are no soft claims.

### `Rule` and `Engine`

```go
type Rule struct {
    Code        string                                // stable ID
    Name        string                                // human-readable name (used in code comments + methodology)
    Section     Section                               // which UI section
    MinSamples  func(snap SignalSnapshot) bool        // sample-size guard — rule does not fire if false
    Compute     func(snap SignalSnapshot) (Insight, bool)
    Methodology string                                // path to methodology page
}

type Engine struct {
    rules []Rule
}

func NewEngine() *Engine {
    return &Engine{rules: catalog}
}

func (e *Engine) Derive(snap SignalSnapshot) []Insight {
    out := make([]Insight, 0, len(e.rules))
    for _, r := range e.rules {
        if !r.MinSamples(snap) {
            continue
        }
        ins, ok := r.Compute(snap)
        if !ok {
            continue
        }
        ins.Code = r.Code
        ins.Section = r.Section
        ins.Methodology = r.Methodology
        out = append(out, ins)
    }
    return out
}
```

### Catalog registration

Each `rules_*.go` file defines its rules and adds them to the package-level `catalog` slice in an `init()` function. The order of registration is the order of evaluation (rules are independent — no rule reads another rule's output).

```go
// rules_age.go
package insights

func init() {
    catalog = append(catalog,
        ruleAgeListedHeadline(),
        ruleMultiplePolicies(),
        rulePermanentBans(),
    )
}

func ruleMultiplePolicies() Rule {
    return Rule{
        Code:    "multiple_retention_policies",
        Name:    "Multiple retention policies",
        Section: SectionRetention,
        MinSamples: func(s SignalSnapshot) bool {
            return s.AgeOfRemoved.Total >= 1000
        },
        Compute: func(s SignalSnapshot) (Insight, bool) {
            p50 := percentileHours(s.AgeOfRemoved, 0.50)
            p90 := percentileHours(s.AgeOfRemoved, 0.90)
            if p50 == 0 {
                return Insight{}, false
            }
            ratio := float64(p90) / float64(p50)
            if ratio < 5.0 {
                return Insight{}, false
            }
            return Insight{
                Headline: fmt.Sprintf(
                    "Half of removed IPs were dropped within %s; the slower 10%% were kept up to %s (%.1f× longer).",
                    formatDuration(p50), formatDuration(p90), ratio,
                ),
                Evidence: map[string]any{
                    "p50_hours":      p50,
                    "p90_hours":      p90,
                    "ratio":          ratio,
                    "removed_total":  s.AgeOfRemoved.Total,
                },
            }, true
        },
        Methodology: "/methodology/multiple-retention-policies",
    }
}
```

**The pattern**: every rule is one function returning a `Rule`, which contains its own `MinSamples` guard, its own `Compute` math, and its own methodology link. Every rule has a corresponding `*_test.go` file with synthetic snapshots covering boundary conditions.

## The 15 starter rules

All rules in this catalog are **deterministic and mathematical**. They use only the data we already have. They never produce verdicts — they state numbers.

### Section: Overview (top "What we noticed" callout)

Only `Certain` rules show up here. Maximum 5 surfaced; if more fire, prioritize by section diversity.

| Code | Inputs | Math | Sample guard | Headline template |
|---|---|---|---|---|
| `R01` `size_variation` | `SizeSeries` | `range = (max-min)/median` | ≥ 50 points | "Over the last {N} updates ({duration}), the list size ranged from {min} to {max}." |
| `R02` `currently_listed_age_p75` | `AgeOfListed` | `p75 = percentileHours(.75)` | ≥ 100 IPs | "75% of currently-listed IPs have been here for at most {p75}." |
| `R03` `currently_listed_age_p100` | `AgeOfListed` | `p100 = max(BucketsHours where Counts > 0)` | ≥ 100 IPs | "Oldest currently-listed IP has been here for {p100}." |
| `R04` `independent` | `Overlaps` | `max(OurShare across overlaps) < 0.10 AND len(overlaps) >= 5` | ≥ 5 compared feeds | "{pct}% of this list's IPs do not appear in any other feed we track." |

### Section: Composition (beside geo/asn/bogons)

| Code | Inputs | Math | Sample guard | Headline template |
|---|---|---|---|---|
| `R05` `country_concentrated` | `TopCountries` | `top3_share = sum(top 3 .Share) > 0.70` | ≥ 100 IPs and ≥ 3 countries | "{c1} ({pct1}%), {c2} ({pct2}%) and {c3} ({pct3}%) account for {top3_pct}% of this list." |
| `R06` `country_diverse` | `TopCountries` | `every country .Share < 0.05 AND count >= 50` | ≥ 100 IPs | "No single country exceeds 5%; this list spans {n_countries} countries." |
| `R07` `single_country` | `TopCountries` | `top1.Share > 0.95` | ≥ 100 IPs | "{country} alone holds {pct}% of this list." |
| `R08` `bogon_present` | `BogonShare` | `> 0` | ≥ 100 IPs | "{n_ips} IPs ({pct}%) are in bogon ranges." |
| `R09` `infrastructure_present` | `InfraShare` | `> 0` | ≥ 100 IPs | "{n_ips} IPs ({pct}%) belong to ASNs on the critical infrastructure list." |

### Section: Retention (beside age charts)

| Code | Inputs | Math | Sample guard | Headline template |
|---|---|---|---|---|
| `R10` `removed_age_p75` | `AgeOfRemoved` | `p75 = percentileHours(.75)` | ≥ 1000 removed | "75% of removed IPs were kept for at most {p75} before being dropped." |
| `R11` `multiple_retention_policies` | `AgeOfRemoved` | `p90/p50 > 5.0` | ≥ 1000 removed | "Half of removed IPs were dropped within {p50}; the slower 10% were kept up to {p90} ({ratio}× longer)." |
| `R12` `permanent_bans` | `AgeOfRemoved` | `p100/p90 > 10.0` | ≥ 1000 removed | "10% of removed IPs were kept up to {p90}; the longest-held was {p100} ({ratio}×)." |

### Section: Trends (beside size/churn charts)

| Code | Inputs | Math | Sample guard | Headline template |
|---|---|---|---|---|
| `R13` `churn_high` | `ChurnSeries` | `median(churn) > 0.50` | ≥ 50 points | "Median churn over the last {N} updates: {pct}% of the list changes per update." |
| `R14` `churn_low` | `ChurnSeries` | `median(churn) < 0.05` | ≥ 50 points | "Median churn over the last {N} updates: {pct}% of the list changes per update." |

### Section: Relationships (beside overlap section)

| Code | Inputs | Math | Sample guard | Headline template |
|---|---|---|---|---|
| `R15` `subset_of` | `Overlaps` | `there exists O: O.OurShare > 0.95 AND O.OlderThanThis` | ≥ 100 IPs | "{pct}% of this list's IPs also appear in {other_name} ({other_category})." |
| `R16` `cross_category_overlap` | `OverlapsByCat` | `OverlapsByCat[c] > 0.30 for some c != own.category` | ≥ 100 IPs and ≥ 3 feeds in c | "Although categorized as {own_category}, this list overlaps {pct}% with {other_category} feeds." |

(That's 16 in the table, not 15. The headline count was approximate — final catalog ships whatever survives review.)

## Sample-size guards — non-negotiable

Every rule must check sample size BEFORE doing its math. The guards are:

| Data type | Minimum to trust |
|---|---|
| `SizeSeries` / `ChurnSeries` (time-series statistics) | 50 points |
| `AgeOfListed` (currently-listed age percentiles) | 100 IPs |
| `AgeOfRemoved` (removed-IP percentiles, multi-policy detection) | 1000 IPs |
| `TopCountries` / `TopASNs` (concentration claims) | 100 IPs |
| `Overlaps` (subset, derivative, independent claims) | 100 IPs in this feed AND 5+ feeds compared |
| `OverlapsByCat` (cross-category claims) | 100 IPs AND 3+ feeds in the target category |

If sample is below the guard, the rule does not fire. The user sees nothing about that claim — neither a hedged version nor a "not enough data" placeholder. **Silence is the right answer.**

## What insights look like in the UI

Two surfaces:

### Surface 1: Overview callout (top of feed page, after the hero, before vitals)

A small "What we noticed" card containing **only `Certain` rules from any section**, max 5 items. Each item is a single sentence with a methodology link.

```
┌────────────────────────────────────────────────────────────┐
│ What we noticed                                             │
│ ───────────────                                             │
│ • Over the last 500 updates (8 yr 3 mo), list size ranged   │
│   from 1.2M to 12.4M.                                  [?]  │
│ • 75% of currently-listed IPs have been here at most 14 d. [?]  │
│ • France (32%), Germany (18%) and Russia (12%) account for  │
│   62% of this list.                                    [?]  │
│ • 87% of this list's IPs do not appear in any other feed    │
│   we track.                                            [?]  │
│ • 42 IPs (0.0001%) belong to ASNs on the critical           │
│   infrastructure list.                                 [?]  │
└────────────────────────────────────────────────────────────┘
```

Each `[?]` is a methodology link, not a tooltip. Tooltips lose users on touch.

### Surface 2: In-context callouts (beside relevant section charts)

When an insight fires for a specific section, a small callout appears next to that section's chart:

```
[ Composition section ]
┌─ Geographic distribution ──────────┬─────────────────────────┐
│                                    │ ⓘ {country} alone holds │
│       ┌──────────────────┐        │   97% of this list.     │
│       │   choropleth      │        │   methodology →         │
│       └──────────────────┘        │                         │
│                                    │                         │
└────────────────────────────────────┴─────────────────────────┘
```

Callouts are visually subdued — small font, muted background, no decoration. They are facts, not banners.

### Critical UX rules

1. **No insight ever uses the words**: "may be", "appears to", "likely", "possibly", "good", "bad", "strong", "weak", "reliable", "unreliable", "trustworthy". Just numbers.
2. **No insight makes a recommendation**: never "you should", never "consider", never "be careful with". Just facts.
3. **No insight stays on the page for more than 2 seconds without context**: every callout has either a methodology link OR is paired with the chart that proves it.
4. **No insight shows a number without its underlying values being inspectable** via the methodology page or the API.

## How rules are added

The bar for a new rule:

1. **Mathematical**: it can be expressed as arithmetic over the SignalSnapshot. No statistical inference, no machine learning, no human judgment.
2. **Sample-size guarded**: the rule must specify the minimum sample needed to trust it.
3. **Tested**: at least 3 test cases — one where it fires correctly, one where it would naively false-positive but is correctly suppressed by the guard, one where it stays silent on insufficient data.
4. **Methodology page**: a Markdown file under `pkg/web/static/methodology/` documenting the formula, the threshold, the data source, and explicitly listing "when this rule would be wrong".
5. **One-line headline**: single sentence, factual, no editorial words. The headline IS the insight.

## Engine integration

```go
// pkg/engine/insights.go (NEW)

func (e *Engine) writeInsights(name string) error {
    snap, err := e.buildSignalSnapshot(name)
    if err != nil {
        return err
    }
    insights := e.insights.Derive(snap)
    payload := insightsJSON{
        Name:     name,
        Computed: e.now().UTC().Unix(),
        Items:    insights,
    }
    data, err := jsonMarshalTabIndent(payload)
    if err != nil {
        return err
    }
    return writeFileAtomic(
        filepath.Join(e.outputDir(), name+"_insights.json"),
        append(data, '\n'),
        0o644,
    )
}

func (e *Engine) buildSignalSnapshot(name string) (insights.SignalSnapshot, error) {
    // Assembles SignalSnapshot from existing engine state:
    //  - SizeSeries / ChurnSeries from history.csv
    //  - AgeOfListed from <name>_retention.json `current`
    //  - AgeOfRemoved from <name>_retention.json `past`
    //  - TopCountries from <name>_<provider>_country.json (default provider: maxmind_geolite2_country)
    //  - TopASNs from <name>_asn_<provider>.json (default provider: maxmind_geolite2_asn)
    //  - BogonShare from <name>_bogons_*.json union
    //  - InfraShare from infrastructure ASN attribution
    //  - Overlaps from <name>_comparison.json
    //  - LastUpdatedTS, ConfiguredFreqMin, DownloadFailures, ClockSkewSeconds from cache.Entry
}
```

The engine writes `<name>_insights.json` after all comparison files are written, in the heavy block. Rebuilds happen via the same fan-out mechanism: when an asn/geo/bogon source updates, all consumer feeds get their insights regenerated.

## API endpoint

```
GET /api/v1/sets/{name}/insights → 200 + insightsJSON
GET /api/v1/sets/{name}/insights → 404 if hidden source
```

Frontend fetches it lazily when the feed detail page loads (alongside the existing metadata fetch).

## Frontend rendering

```js
// app.js — load insights when feed page opens
async function loadInsights(name) {
  const r = await fetch(`/api/v1/sets/${name}/insights`);
  if (!r.ok) { this.insights = []; return; }
  const data = await r.json();
  this.insights = data.items;
}

// Group by section for in-context rendering
function insightsForSection(section) {
  return this.insights.filter(i => i.section === section);
}

// Top "What we noticed" callout — limited to 5 items, prioritized by section diversity
function topInsights() {
  const groups = {};
  for (const i of this.insights) {
    groups[i.section] = groups[i.section] || [];
    groups[i.section].push(i);
  }
  // Round-robin pick across sections to maximize diversity
  const sections = Object.keys(groups);
  const out = [];
  let idx = 0;
  while (out.length < 5 && sections.some(s => groups[s].length > 0)) {
    const s = sections[idx % sections.length];
    if (groups[s].length > 0) out.push(groups[s].shift());
    idx++;
  }
  return out;
}
```

## Tests

Each rule gets a `_test.go` file with at least 3 cases:

```go
// rules_age_test.go
func TestRuleMultiplePolicies_Fires(t *testing.T) {
    // Bimodal removal age distribution: many removed in 1 day, many in 30 days
    snap := buildSnap().withRemovedAge(map[int]uint64{
        24:  500, 48:  300, 72:  200, // most removed within 3 days
        720: 100, 744: 80, 768: 60,    // some kept ~30 days
    })
    insights := NewEngine().Derive(snap)
    require.Contains(t, insightCodes(insights), "multiple_retention_policies")
}

func TestRuleMultiplePolicies_DoesNotFireOnUniformDistribution(t *testing.T) {
    snap := buildSnap().withRemovedAge(uniformHistogram(24, 1000))
    insights := NewEngine().Derive(snap)
    require.NotContains(t, insightCodes(insights), "multiple_retention_policies")
}

func TestRuleMultiplePolicies_DoesNotFireOnTinySample(t *testing.T) {
    snap := buildSnap().withRemovedAge(map[int]uint64{24: 5, 48: 3, 720: 2})
    insights := NewEngine().Derive(snap)
    require.NotContains(t, insightCodes(insights), "multiple_retention_policies")
}
```

The test helpers (`buildSnap()`, `uniformHistogram()`, `insightCodes()`) live in a single `testhelpers_test.go` file.

Engine-level test (`insights_test.go`) runs the full catalog against a known-good snapshot and asserts exactly which rules fire. **This test breaks every time we change a threshold** — that's a feature, not a bug. It forces a conscious rethreshold step.

## Methodology pages

For each rule, a Markdown file at `pkg/web/static/methodology/<rule_slug>.md`:

```markdown
# Multiple retention policies

When a list shows two distinct retention windows for removed IPs (a fast
window for ephemeral entries, a slower window for confirmed bad actors),
this rule reports it.

## How we calculate this

For each IP that was removed from the list, we record how long it was
listed before removal. We compute the 50th and 90th percentiles of these
durations. If the 90th percentile is more than 5 times longer than the
50th, the list has two distinct retention windows.

## Threshold

p90 / p50 > 5.0

## Sample size

Requires at least 1000 removed IPs. Below this, the percentiles are too
noisy to trust.

## When this rule would be wrong

- A list with very few removals (most IPs are kept indefinitely) — this
  is mitigated by the 1000-removed minimum.
- A list with a smooth log-normal distribution of retention times — the
  ratio could exceed 5.0 without a true bimodal split. We accept this
  false positive in exchange for catching the more common bimodal case.
```

The methodology page is generated automatically from the rule definition by `pkg/insights/methodology_writer.go`. The "When this rule would be wrong" section is hand-written in the rule's Go file as a comment, then extracted into the page.

## Phased delivery

| Phase | What ships | Risk |
|---|---|---|
| **Phase 3a** (with website redesign) | All 15 deterministic rules. `pkg/insights/` package. JSON output. API endpoint. UI top-callout + in-context callouts. | Low — every rule is pure arithmetic. |
| **Phase 3b** (after Phase 3a deploys) | Brainstorm tool runs once over all 200 feeds × 2 local LLMs. Output read by humans. Patterns we accept get codified as new deterministic rules in `pkg/insights/`. | Tool is throwaway. No production dependency on LLMs. |
| **Phase 4** (later) | More rules as we discover them. Possibly: tighter integration between insights and visualization (e.g., when `multiple_retention_policies` fires, the retention chart auto-highlights the bimodal split). | Iterative. |

## Out of scope

- LLMs at runtime, ingestion, or scheduled re-evaluation. **Never.**
- Confidence labels in the UI. **Never.**
- Editorial verdicts ("this list is reliable / unreliable"). **Never.**
- Recommendations ("you should block / allow"). **Never.**
- Maintainer grades. **Never.**
- Overlapping rules. Each rule fires on its own merits.
- Rule ordering effects. Rules are independent — order in the catalog only affects which fires first when both ARE valid (used in the diversity-based top 5 selection).
- A "rules dashboard" admin UI for tweaking thresholds at runtime. Thresholds live in code, get changed in PRs.

## Risks

1. **Threshold drift**: rules that fire too often or too rarely degrade trust. Mitigation: every rule has a test corpus; thresholds change in PRs; the engine-level test catches surprise changes.
2. **Headline phrasing**: a single bad headline ("87% of IPs are unique" sounds editorial; "87% of this list's IPs do not appear in any other feed we track" is just a fact). Mitigation: the "no editorial words" rule + every PR adding a rule must justify the headline phrasing.
3. **Sample-size erosion**: as new rules get added, sample-size guards must be enforced consistently. Mitigation: code review + the `MinSamples` field is mandatory in `Rule`.
4. **Rule sprawl**: the catalog could balloon. Mitigation: the bar for a new rule is high; the brainstorm step happens AFTER Phase 3a so we have a baseline to compare proposed additions against.
5. **Performance**: 200 feeds × 16 rules each = 3200 rule evaluations per heavy block. Each rule is O(snapshot size) at worst. Should take well under a second on the production daemon. Profile if it turns out worse.

## Done when

- `pkg/insights/` package exists with the 16 starter rules
- Each rule has a `_test.go` with the 3 test cases
- `pkg/engine/insights.go` writes `<name>_insights.json` for every feed
- `/api/v1/sets/{name}/insights` endpoint returns the JSON
- Frontend reads + renders insights in top callout + in-context callouts
- Methodology pages exist for every rule
- `go test ./pkg/insights/...` passes
- Visual smoke test: navigate to ~5 different feeds, confirm insights look right
