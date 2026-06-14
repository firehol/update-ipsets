# Common Issues

You will learn how to diagnose and fix the most common operational problems.

## All feeds show as unavailable

**Symptom:** Every feed in the admin UI shows `unavailable` status. No downloads succeed.

**What to check:**

1. Network connectivity — can the server reach the internet?
   ```bash
   curl -I https://iplists.firehol.org
   ```
2. DNS resolution — can the server resolve hostnames?
   ```bash
   dig google.com
   ```
3. Firewall rules — is outbound HTTPS allowed?
4. Proxy configuration — if you use a proxy, set `HTTP_PROXY` and `HTTPS_PROXY` environment variables in the systemd unit

**Fix:** Resolve the network issue. Feeds recover automatically once connectivity is restored.

## Admin UI returns 401 or 503

**Symptom:** Accessing `/admin` or `/api/v1/admin/*` returns HTTP 401 Unauthorized or 503 Service Unavailable.

**What to check:**

1. Are admin credentials configured? Missing credentials return 503.
   ```bash
   systemctl show update-ipsets | grep Environment
   ```
   Look for `UPDATE_IPSETS_ADMIN_USER` and `UPDATE_IPSETS_ADMIN_PASSWORD`.
2. Are you sending the correct credentials in the request? Wrong credentials return 401.
3. Are you hitting the correct listener? If `--admin-listen` is set, admin routes return 404 on the public listener.

**Fix:** Set the credential environment variables and restart:

```bash
sudo systemctl edit update-ipsets
# Add:
# [Service]
# Environment="UPDATE_IPSETS_ADMIN_USER=admin"
# Environment="UPDATE_IPSETS_ADMIN_PASSWORD=your-secret"
sudo systemctl restart update-ipsets
```

## Feed data is empty

**Symptom:** A feed downloads successfully but produces zero IPs.

**What to check:**

1. Is the output family correct? A feed configured as `ipset` that receives CIDR input will expand CIDRs to individual IPs. A feed configured as `netset` renders single IPs as `/32` prefixes.
2. Is the processor pipeline correct? Check that the configured processors match the upstream format.
3. Does the upstream feed actually contain IPs? Download it manually and inspect:
   ```bash
   curl -s <feed-url> | head -20
   ```

**Fix:** Adjust the processor configuration or output family in the YAML catalog.

## High memory usage

**Symptom:** Process RSS grows beyond expectations.

**What to check:**

1. Is `GOMEMLIMIT` set? The installed unit sets it by default. Custom units or
   drop-ins may override it.
   ```bash
   systemctl show update-ipsets | grep GOMEMLIMIT
   ```
2. Are there unusually large feeds? Check feed sizes in the admin UI.
3. Is background work queued? Entity artifact rebuilds can be memory-intensive.

**Fix:**

```bash
# Set a memory limit in the systemd unit
sudo systemctl edit update-ipsets
# Add:
# [Service]
# Environment="GOMEMLIMIT=512MiB"
# MemoryHigh=512M
# MemoryMax=768M
sudo systemctl restart update-ipsets
```

## Integrity keeps finding issues

**Symptom:** The integrity panel shows findings that keep reappearing.

**What to check:**

1. Is the pipeline actively processing? Integrity reports `in_progress` during active runs. Findings during active processing are transient.
2. Are the findings the same each time, or different? Same findings after the pipeline settles indicate a real issue.

**Fix:** Wait for the pipeline to settle. If findings persist after processing completes, use the integrity recovery action. If recovery does not fix the issue, check the logs for the specific error.

## Feeds not updating

**Symptom:** Feeds show old timestamps and no recent activity.

**What to check:**

1. Is the feed enabled? Check the enable marker in the admin UI.
2. Is the scheduler moving work? Check admin status fields `queues`, `metrics.download_started`, `metrics.download_finished`, and `metrics.processing_batches_completed`.
3. Is `--enable-all` configured? Without it, only explicitly enabled feeds are active.
4. Is the feed archived? Archived feeds stop automatic scheduling.

**Fix:**

```bash
# Enable a specific feed
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/enable

# Trigger all due work
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/run
```

If using `--enable-all`, verify the daemon was started with that flag:

```bash
ps aux | grep update-ipsets
```
