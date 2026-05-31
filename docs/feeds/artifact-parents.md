# Artifact Parents

You will learn how to define a downloadable artifact that produces one or more child feeds, how children reference their parent, and how artifact-backed feeds differ from source feeds.

## What an artifact parent is

An artifact parent is a downloadable file that is not itself a public feed. It exists to produce one or more child feeds, each extracting a specific subset from the downloaded artifact.

Use artifacts when an upstream publishes a single large file containing multiple categories of data, and you want to split it into separate feeds.

## Defining an artifact

Artifacts live in the `artifacts/` directory. Each artifact has its own YAML file.

```yaml
artifacts:
  dronebl:
    type: dronebl_buildzone
    frequency: 60
    max_download_size: 268435456
    info: '[DroneBL.org](https://dronebl.org) shared buildzone download used to derive the DroneBL family of IP feeds.'
    maintainer: DroneBL.org
    maintainer_url: https://dronebl.org
    rsync_url: rsync://firehol@rsync.dronebl.org/dronebl/
```

Key fields:

| Field | Description |
|-------|-------------|
| `type` | Artifact family — controls how the download is parsed and split |
| `frequency` | Minutes between automatic downloads |
| `max_download_size` | Override the global max download size for this artifact, in bytes |
| `info` | Description for the admin UI |
| `maintainer` | Artifact source attribution |

## Referencing an artifact from a child feed

Child feeds reference their parent using an `artifact://` URL:

```yaml
sources:
  dronebl_anonymizers:
    url: artifact://dronebl?parts=http_proxies,socks_proxies,web_page_proxies,wingate_proxies,proxychains
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - $CAT_CMD
    processor_raw: $CAT_CMD
    category: anonymizers
    info: '[DroneBL.org](https://dronebl.org) list of open proxies'
    maintainer: DroneBL.org
    maintainer_url: https://dronebl.org
```

The URL format is:

```
artifact://<artifact-name>?parts=<comma-separated-parts>
```

- `<artifact-name>` references a configured artifact parent.
- `parts=` lists one or more named deliveries from that artifact.
- The child does not need to know the artifact's internal directory structure.

## Cadence rules

Artifact-backed children do not own an independent fetch cadence. Set `frequency: 0` on child feeds.

The artifact parent owns the download cadence. When the parent downloads new data, the scheduler queues all children for reprocessing.

## Multiple children, one parent

A single artifact can produce many child feeds. Each child selects different parts:

```yaml
# dronebl_anonymizers
url: artifact://dronebl?parts=http_proxies,socks_proxies,web_page_proxies

# dronebl_worms_bots
url: artifact://dronebl?parts=worms,bots
```

Each child is an independent feed with its own name, category, metadata, and output type.
