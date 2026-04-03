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

Returns a JSON object mapping every public feed name to its current file content. This is a single-request way to fetch all feed data at once.

Use this for batch synchronization. The response can be large depending on the number and size of active feeds.
