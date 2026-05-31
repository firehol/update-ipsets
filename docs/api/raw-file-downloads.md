# Raw File Downloads

You will learn how to download raw .ipset and .netset files directly from the daemon.

## File endpoint

```
GET /files/{name}.{ipset|netset}
```

Serves the committed raw feed file directly from disk. The path uses the feed name and file extension matching the feed's output type.

This is the simplest way to download a feed for use with firewall tools like `ipset restore` or `iptables`.

Example:

```
GET /files/firehol_level1.netset
```

Response (`text/plain`):

```
1.2.3.0/24
10.20.30.0/24
192.168.1.1
```

## Compatibility route

```
GET /{name}.{ipset|netset}
```

The daemon also serves direct root-level `.ipset` and `.netset` paths for compatibility
with bash-era scripts and simple static-file consumers.

Example:

```
GET /firehol_level1.netset
```

The eligibility rules and response body are the same as `/files/{name}.{ipset|netset}`.
For new automation, prefer `/files/{name}.{ipset|netset}` or the API data endpoint.

## Eligibility

Only public, redistributable, non-archived feeds are served. The endpoint returns 404 for:

- hidden feeds
- non-redistributable feeds
- provider-only datasets
- feeds that do not have a committed file

## Difference from the API data endpoint

The `/api/v1/sets/{name}/data` endpoint and `/files/{name}.{ext}` serve the same content. The difference is routing:

- `/files/` uses the feed name with file extension — convenient for direct downloads and scripting
- `/api/v1/sets/{name}/data` uses the API path without extension — consistent with other API endpoints

## Bulk download

```
GET /all-ipsets.json
```

Returns the legacy public feed catalog metadata as JSON. Each item identifies one public feed and includes fields such as feed name, category, maintainer, timestamps, current IP count, and error count.

Use this when existing automation expects the bash-era `all-ipsets.json` listing. It is metadata, not a bulk dump of every feed body. Download raw feed bodies through `/files/{name}.{ipset|netset}`, `/{name}.{ipset|netset}`, or `/api/v1/sets/{name}/data`.
