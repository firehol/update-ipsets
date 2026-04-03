# Download Lifecycle

You will learn the steps a feed goes through during download, what each status outcome means, and how the downloader handles retries and large responses.

## Steps

Every feed in the downloader loop follows this sequence:

1. **Schedule check** — is this feed due? The downloader evaluates cadence, retry backoff, and any pending manual actions.
2. **Fetch** — acquire source material from upstream (HTTP, local file, or synthetic composition).
3. **Compose feed body** — normalize the raw source into a canonical plain-text feed body in the configured output family (`ipset` or `netset`).
4. **Compare with current** — hash the canonical feed body and compare against the latest local version. The comparison is semantic (same IPs/CIDRs), not byte-by-byte.
5. **Stage if changed** — write the new feed body to a `.new` file on disk. This is the handoff point to the processing loop.

If the content is unchanged, the downloader reports `same` and stops. No staging happens. No processing is queued.

## Status outcomes

| Status | Meaning |
|---|---|
| `ok` | New or changed content obtained and staged. |
| `not_modified` | Upstream confirmed no change (e.g. HTTP 304). |
| `same` | Downloaded successfully, but content matches the current local version. |
| `empty` | Downloaded successfully, but no IPs/CIDRs found in the source. |
| `skipped` | Not checked this cycle (not due, wrong cadence, etc.). |
| `failed` | Download or composition failed. |

## Retry behavior

Failed downloads get retried automatically:

- First retry after `cadence / 16`.
- Retry interval doubles on each failure.
- Capped at the configured ordinary cadence.
- Once a feed reaches `unmaintained` health, retries continue doubling up to one month.

Retries apply only to hard download failures. `same`, `not_modified`, and successful `empty` do not trigger retries.

## Same-body detection

The downloader normalizes upstream content into a canonical form first, then compares the result against the latest local feed body. This comparison uses a content hash of the normalized IP/CIDR set, not raw upstream bytes.

This means formatting differences (trailing whitespace, comment changes, line ordering) do not trigger unnecessary processing.

## Spill to disk

HTTP responses stream directly to temporary files on disk. The daemon does not hold the full response body in RAM during download. This keeps memory use bounded even for large feeds.

## Feed families in the downloader

Different feed families take different paths through the downloader:

- **Plain feeds** — fetch upstream, normalize, compare, stage.
- **History derivatives** — compose from parent body plus retained history snapshots. No upstream fetch.
- **Merges** — compose from local feed bodies of enabled inputs. No upstream fetch.
- **Artifact parents** — fetch the parent artifact, then materialize child feed bodies from it.
- **Provider databases** — fetch and stage supporting datasets (ASN, GeoIP, bogon). May trigger broad reprocessing of all feeds that depend on them.

## See also

- [Feed Status Reference](feed-status-reference.md) — complete list of status values
- [Pipeline Overview](pipeline-overview.md) — how download fits into the full pipeline
