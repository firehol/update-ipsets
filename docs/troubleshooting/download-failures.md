# Download Failures

You will learn what each download failure means, what to check, and how to fix it.

## Common HTTP errors

### 403 Forbidden

The server rejected the request. This usually means:

- The feed URL changed and the old URL no longer accepts requests
- The feed now requires authentication
- Your IP address is rate-limited or blocked

**What to check:** Try opening the feed URL in a browser. If it requires login, check whether the provider offers an API key or alternative URL. Update the source URL in the configuration catalog.

### 404 Not Found / 410 Gone

The feed URL no longer exists. The upstream provider removed or relocated it.

**What to check:** Visit the provider's website to find the current URL. If the feed is permanently gone, consider disabling it or removing it from the catalog.

### 429 Too Many Requests

You are hitting the upstream rate limit.

**What to check:** Reduce the feed's configured frequency. The `frequency` field is minutes, so changing a feed from `60` to `1440` moves it from hourly checks to daily checks. Most providers document their rate limits.

The daemon also has built-in retry backoff — it waits longer between retries after each failure. If 429 errors are occasional, the daemon recovers on its own.

### 5xx Server Error

The upstream server has a temporary problem.

**What to check:** Usually resolves on its own. The daemon retries with exponential backoff. If errors persist for days, the upstream may have a sustained outage. Check the provider's status page.

## DNS failures

The hostname in the feed URL cannot be resolved.

```
error: lookup feed.example.com: no such host
```

**What to check:**

- DNS is working on the server: `dig feed.example.com`
- The hostname is still correct in the configuration
- The domain has not expired or been taken down

## Timeouts

The connection or read operation timed out.

```
error: context deadline exceeded
```

**What to check:**

- Network connectivity to the host: `curl -I <feed-url>`
- Whether the feed is unusually large and needs a higher timeout
- Whether a proxy or firewall is slowing the connection

Timeouts are often transient. The daemon retries automatically.

## Oversized downloads

The download exceeded `max_download_size` (default 100 MB).

```
error: download exceeded max size (104857600 bytes)
```

**What to check:**

- Verify the URL points to a feed, not a full database dump
- Some providers offer lightweight variants of their feeds
- If the feed is genuinely large, raise `max_download_size` in the configuration

## SSL errors

Certificate validation failed.

```
error: tls: failed to verify certificate: x509: certificate signed by unknown authority
```

**What to check:**

- The system CA certificates are current: `update-ca-certificates` (Debian/Ubuntu) or `update-ca-trust` (RHEL/CentOS)
- The feed's SSL certificate is valid and not expired
- There is no transparent proxy intercepting HTTPS traffic

## Checking download status

Use the admin API to see which feeds are failing:

```bash
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds | \
  jq '.[] | select(.last_status == "download_failed") | {name, last_error}'
```

## Forcing a retry

After fixing the underlying issue, trigger a manual recheck:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/recheck
```

This bypasses the schedule and fetches the feed immediately.
