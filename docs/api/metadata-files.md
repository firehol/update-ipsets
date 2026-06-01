# Metadata Files

You will learn about the public metadata files the daemon serves: robots.txt, sitemaps, and llms.txt.

## robots.txt

```
GET /robots.txt
```

Returns the crawler policy file. It points crawlers to the sitemap and discourages access to live query endpoints.

Disallowed paths:

- `/api/v1/search`
- `/api/v1/query`
- `/api/v1/compose`
- `/api/v1/client-ip`
- per-feed search endpoints

The robots.txt file is an advisory crawler hint, not a security control. It does not list admin paths or private local paths.

## Sitemap index

```
GET /sitemap.xml
```

Returns a Sitemaps.org sitemap index. This index links to individual sitemap shard files.

Each shard uses absolute URLs and the Sitemaps.org XML namespace. Shards stay below 45,000 URLs to remain well under the 50,000-URL sitemap protocol limit.

## Sitemap shards

```
GET /sitemap-*.xml
```

Individual sitemap shard files. Each shard covers one category of pages:

- feed detail pages — one URL per public feed
- country detail pages — one URL per country in the public index
- ASN detail pages — one URL per ASN in the public index
- maintainer detail pages — one URL per public maintainer
- index pages — homepage, countries, ASNs, maintainers, methodology

Sitemaps do not include admin routes, API routes, raw file downloads, or private runtime details.

## llms.txt

```
GET /llms.txt
```

Returns a concise Markdown file for AI agents and automated tools. It links to public pages, methodology pages, public API indexes, and the feed catalog.

The file follows the emerging `llms.txt` convention for curated AI-readable site context. It does not expose admin routes, authenticated operations, local filesystem paths, or private runtime details.

Example content structure:

```markdown
# FireHOL IP Lists

> Public cybercrime IP feed observatory for discovering, comparing, and consuming maintained IP blocklists.

## Primary Pages

- / — homepage with IP lookup and feed explorer
- /countries — country index
- /asns — ASN index
- /maintainers — maintainer index
- /methodology — methodology index

## Public APIs

- /api/v1/status — high-level public service state
- /api/v1/categories — public category registry
- /api/v1/sets — feed catalog
- /api/v1/search — IP lookup
- /api/v1/countries — country index
- /api/v1/asns — ASN index
- /api/v1/maintainers — maintainer index
- /api/v1/methodology — methodology index
- /api/v1/compose — compose example when at least one public feed exists

## Feed Surfaces

- /all-ipsets.json — legacy feed catalog JSON
- /api/v1/sets — public feed API index
- /ipsets/{name} — example public feed detail page when at least one public feed exists

## Optional

- /sitemap.xml — XML sitemap for public pages
- /robots.txt — crawler policy and sitemap pointer
```
