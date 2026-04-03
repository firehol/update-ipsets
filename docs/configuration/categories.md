# Categories

You will learn how categories organize feeds for public browsing, what fields each category defines, and how to control category visibility.

## What categories do

Categories group feeds into a public taxonomy. Each feed belongs to exactly one category. The public website uses categories for browsing, filtering, and color-coding feeds.

## Category fields

Every category in `categories.yaml` defines these fields:

| Field | Type | Description |
|-------|------|-------------|
| `label` | string | Human-readable name shown in the UI |
| `description` | string | One-sentence explanation of what feeds in this category track |
| `color` | string | CSS hex color for UI badges, tags, and charts |
| `sort_order` | integer | Lower numbers appear first in the public browsing list |
| `public` | boolean | Whether the category appears on the public website. Defaults to `true` when omitted. |

## Example

```yaml
categories:
  intrusion:
    label: Intrusion
    description: IPs observed initiating hostile access attempts against exposed services, including brute force, exploitation, and active attack traffic.
    color: "#dc2626"
    sort_order: 10

  anonymizers:
    label: Anonymizers
    description: IPs whose main significance is hiding origin or bypassing policy, including Tor exits, VPN exits, open proxies, and relay infrastructure.
    color: "#0891b2"
    sort_order: 50

  geolocation:
    label: Geolocation
    description: IP-to-country datasets used to attribute feeds geographically.
    color: "#0f766e"
    sort_order: 100
    public: false
```

## Public vs non-public categories

Omit `public` or set it to `true` — the category appears on the public site and in public API responses.

Set `public: false` — the category is valid configuration for system roles but does not appear in public browsing. For example, `geolocation` and `asn` categories hold provider databases that enrich other feeds but are not themselves public threat feeds.

The public website derives category visibility from configuration, not from hardcoded category names. If you add a new non-public category, set `public: false` explicitly.

## Shipped categories

The shipped catalog defines these categories in sort order:

| Category | Label | Public |
|----------|-------|--------|
| `intrusion` | Intrusion | yes |
| `malware_infrastructure` | Malware Infrastructure | yes |
| `messaging_abuse` | Messaging Abuse | yes |
| `service_abuse` | Service Abuse | yes |
| `anonymizers` | Anonymizers | yes |
| `scanners` | Scanners | yes |
| `policy_risk` | Policy / Risk | yes |
| `provider_infrastructure` | Provider Infrastructure | yes |
| `special_use` | Special Use | yes |
| `geolocation` | Geolocation | no |
| `asn` | ASN | no |

## Adding a category

Add a new entry to `categories.yaml`. Choose a unique key (used in source files as the `category:` field), set the label, description, color, and sort order. Reload the daemon with SIGHUP.

## Category and health thresholds

Categories also influence feed health classification. `runtime.yaml` can define per-category cadence thresholds under `feed_health_category_thresholds`. This lets fast-changing categories like `intrusion` use tighter healthy/risky bounds than slow-changing categories like `special_use`.
