# Filesystem Layout

You will learn where update-ipsets stores its files after installation, what each directory contains, and which directories grow over time.

## Installed directory tree

`./install.sh` creates the top-level directories. The daemon creates sub-directories at runtime as feeds are processed. After the daemon has been running, the tree looks like this:

```
/opt/update-ipsets/
├── bin/
│   └── update-ipsets              # The daemon binary (UI embedded)
├── etc/
│   └── config/                    # Feed catalog (YAML files)
├── data/
│   ├── .cache.json                # Feed state cache
│   ├── {feed}.ipset               # Committed IP set files
│   ├── {feed}.netset              # Committed network set files
│   ├── {feed}.source              # Raw upstream downloads (retained for debugging)
│   ├── {feed}.setinfo             # Per-feed human-readable summary
│   ├── {feed}.enabled             # Source enable markers
│   ├── history/
│   │   └── {parent}/              # History snapshots per feed
│   │       └── {timestamp}.set
│   └── errors/                    # Download error logs
├── cache/
│   └── scheduler-state.json       # Scheduler/runtime ledger
├── lib/
│   ├── {feed}/
│   │   ├── latest                 # Binary snapshot of current IP ranges
│   │   ├── history.csv            # Append-only history ledger
│   │   ├── changesets.csv         # Added/removed IP ledger
│   │   ├── retention.csv          # Removal-life ledger
│   │   ├── retention.json         # Structured retention summary
│   │   ├── histogram              # Bash-compatible histogram cache
│   │   └── new/                   # Retention cohort snapshots
│   │       └── {timestamp}
│   ├── geolocation/
│   │   └── {provider}.source      # Geolocation provider databases
│   ├── asn/
│   │   └── {provider}/source      # ASN provider databases
│   ├── artifacts/
│   │   └── {artifact}/            # Artifact parent local storage
│   ├── entities/                  # Private entity sidecars
│   │   ├── feeds/{feed}.json      # Per-feed country/ASN contributions
│   │   ├── countries/{CODE}.json  # Country-detail sidecars
│   │   └── asns/{ASN}.json        # ASN-detail sidecars
│   └── critical_infrastructure/   # Critical reference state
├── web/
│   ├── index.json                 # Public catalog index
│   ├── all-ipsets.json            # Full public feed listing
│   ├── home/
│   │   └── aggregates.json        # Homepage aggregate payload
│   ├── {feed}.json                # Per-feed public metadata
│   ├── {feed}_history.csv         # Public history CSV
│   ├── {feed}_comparison.json     # Pairwise overlap data
│   ├── {feed}_insights.json       # Deterministic insights
│   ├── countries/
│   │   ├── index.json             # Country listing
│   │   └── {CODE}.json            # Per-country detail
│   ├── asns/
│   │   ├── index.json             # ASN listing
│   │   └── {ASN}.json             # Per-ASN detail
│   ├── files/                     # Downloadable .ipset/.netset files
│   ├── sitemap.xml                # Public sitemap
│   ├── robots.txt                 # Crawler policy
│   └── llms.txt                   # AI agent map
├── run/
│   └── update-ipsets.lock         # Runtime lock file
└── tmp/                           # Scratch space for in-progress writes
```

## What each directory does

### `bin/` — the binary

Contains the single `update-ipsets` executable. The web UI is embedded at compile time. No external static files needed.

### `etc/config/` — feed catalog

The YAML configuration directory that defines all feeds, sources, merges, and provider settings. The installer deploys this from the repository's `configs/firehol/` directory.

On reinstall, the installer backs up the existing config (if changed) and deploys a fresh copy. Your customizations should go through the YAML catalog or drop-in environment variables — not by editing files that the installer overwrites.

### `data/` — feed bodies and state

Contains the committed text-format IP sets, raw upstream downloads, enable markers, and the feed state cache. This is the authoritative location for "what the daemon knows about each feed right now."

Files with a `.new` suffix are staged inputs waiting for the processing engine to claim them. Files with a `.processing` suffix are actively being processed. Both survive restarts.

### `cache/` — runtime caches

Holds scheduler state and other runtime caches that are not feed bodies. Safe to delete — the daemon rebuilds the contents on startup.

### `lib/` — binary sets and analysis data

The heaviest directory. Contains binary range snapshots, history ledgers, retention analysis, provider databases (GeoIP, ASN), and entity sidecars. Each feed gets its own subdirectory under `lib/`.

Provider databases under `lib/geolocation/` and `lib/asn/` are downloaded periodically according to the catalog configuration.

### `web/` — published public artifacts

All precomputed JSON, CSV, XML, and text files served by the public website. The daemon writes these during processing and the HTTP server reads them on request. Public page views never trigger recomputation — they read files from this directory.

### `run/` — runtime coordination

Lock files for process coordination. Cleared automatically on startup.

### `tmp/` — temporary scratch space

Incomplete writes, download spills, and extraction intermediates. Safe to delete while the daemon is stopped.

## Disk usage considerations

Two directories grow with catalog size and uptime:

- **`lib/`** grows per feed. Each feed accumulates history ledgers, retention cohorts, and analysis data. Provider databases add their own size (GeoIP databases can be 50–100MB each).
- **`web/`** grows with the number of published feeds and entity pages. Each feed produces multiple JSON/CSV artifacts. Country and ASN pages add more files.

For a deployment with 200+ feeds, plan for 1–5GB under `lib/` and 100–500MB under `web/` after the initial processing run completes. Growth after that depends on retention cohort accumulation.

Monitor disk usage:

```bash
du -sh /opt/update-ipsets/{lib,web,data}
```

## File staging model

The daemon uses a consistent staging convention to prevent partial writes from corrupting committed state:

1. New data writes to a `.new` sibling or a temp file under `tmp/`
2. On success, the new file replaces the committed file atomically (rename)
3. On failure, the committed file remains untouched

This means crashes and restarts never leave committed files in a partial state.
