# Memory Planning

You will learn how the daemon uses memory, how to set limits, and how to size resources for your feed catalog.

## Out-of-core design

The daemon avoids keeping the whole published catalog in RAM. It uses three techniques to keep routine serving and comparison memory low:

- **File-backed binary sets.** Each feed stores its current IP ranges as a binary snapshot on disk. Query and comparison operations open these files with memory-mapped I/O or positioned reads and perform binary search directly on the file. The long-lived public-serving path does not need to keep every feed's range array on the Go heap.

- **Streaming set operations.** Union, intersection, exclusion, diff, and overlap counting use two-pointer sweeps over iterators. Memory usage stays constant regardless of input size.

- **Streaming downloads and mostly streaming processing.** HTTP responses stream to temp files instead of buffering in memory. Processor steps with streaming implementations chain as readers; whole-input processors such as JSON/XML extraction, zip extraction, regex extraction, and hostname-resolution batches temporarily materialize the active intermediate before streaming can continue.

During feed processing, one worker can still hold the active normalized range set and rendered canonical output in memory before committing the binary snapshot. Plan capacity around the largest active source, the number of processing workers, and any whole-input processor steps, not only the total catalog size.

## GOMEMLIMIT

Set `GOMEMLIMIT` to tell the Go runtime how much memory it should target. This is a soft limit — it makes the garbage collector more aggressive and prompts the runtime to return memory to the OS. It does not kill the process.

```bash
GOMEMLIMIT=512MiB update-ipsets daemon
```

In a systemd drop-in:

```ini
[Service]
Environment="GOMEMLIMIT=512MiB"
```

## Cgroup memory limits

Use systemd's memory controls for hard enforcement:

```ini
[Service]
MemoryHigh=512M
MemoryMax=768M
Environment="GOMEMLIMIT=512MiB"
```

How the two limits interact:

- **MemoryHigh** (512M): The kernel starts reclaiming memory and throttling the process at this threshold. The daemon slows down but keeps running.
- **MemoryMax** (768M): The kernel kills the process if it reaches this threshold. Set it above MemoryHigh to give headroom for transient spikes.
- **GOMEMLIMIT** (512MiB): The Go GC cooperates by reducing its own footprint before the kernel needs to intervene.

This combination produces "degrade under pressure" behavior: the daemon gets slower instead of crashing.

## Sizing guidance

These are starting points. Monitor actual usage and adjust.

| Catalog size | MemoryHigh | MemoryMax | GOMEMLIMIT |
|---|---|---|---|
| Small (< 100 feeds) | 256M | 384M | 256MiB |
| Medium (100–300 feeds) | 512M | 768M | 512MiB |
| Large (300+ feeds) | 768M–1G | 1G–1.5G | 768MiB |

The installed unit ships with `MemoryHigh=1536M`, `MemoryMax=2G`, and
`GOMEMLIMIT=1536MiB`. For most deployments this is generous. Lower these values
if the server shares resources with other services.

## Monitoring memory usage

Check current memory consumption:

```bash
systemctl status update-ipsets
```

For detailed breakdown, query the admin status endpoint:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://127.0.0.1:18889/api/v1/admin/status
```

The response includes Go runtime memory statistics and process-level RSS under `system`.

If you have Netdata running on the same host, use Netdata's normal host/process charts for memory and CPU. OpenTelemetry export adds application operation metrics, but process resource snapshots remain easiest to inspect through the admin status API.

## When to increase limits

Raise limits if you observe:

- Frequent restarts from OOM kills (check `journalctl --namespace=iplists -u update-ipsets` for "OOM" or exit code 137)
- Sustained GC pressure causing slow responses during feed updates
- The daemon approaching MemoryHigh during large catalog processing runs

The out-of-core design means routine serving memory grows slowly with catalog size. Most long-lived feed data stays on disk. Limits mainly need to cover:
- The Go runtime and HTTP server overhead
- Temporary in-memory processing for non-streamable sources (JSON/XML extraction, zip archives, regex extraction, hostname-resolution batches)
- Active canonical feed parsing and rendering for each processing worker
- Small copy buffers during concurrent feed fetches; downloaded bodies are staged on disk
