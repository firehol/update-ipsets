# Configuration Concepts

You will learn how the update-ipsets catalog is organized, how individual YAML files combine into a working configuration, and where each type of definition lives on disk.

## Catalog as a directory

The catalog is a directory of YAML files — not one monolithic file. Each feed, merge, and artifact has its own file. The loader reads all fragments recursively, merges them, normalizes them, and validates the result.

## Directory structure

```
configs/firehol/
  runtime.yaml          Daemon behavior, concurrency, paths, health thresholds
  categories.yaml       Feed categories for public browsing
  defaults.yaml         Provider defaults (ASN, GeoIP)
  renames.yaml          Old-name → new-name aliases
  deleted.yaml          Historical names that are permanently retired
  critical_asn_context.yaml  Secondary ASN context entries

  sources/              One YAML per feed, grouped by category subdirectory
    intrusion/
      dshield.yaml
      feodo.yaml
      ...
    scanners/
      misp_shodan_scanners.yaml
      ...
    special_use/
      bogons.yaml
      ...
    asn/
      maxmind_geolite2_asn.yaml
      ...
    geolocation/
      geolite2_country.yaml
      ...
    ...

  merges/               One YAML per merge feed
    firehol_level1.yaml
    firehol_level2.yaml
    ...

  artifacts/            One YAML per downloadable artifact parent
    dronebl.yaml
    ...
```

## How loading works

1. The loader walks the catalog directory recursively.
2. Every YAML fragment is read and merged into a single configuration tree.
3. The merged tree is normalized: outputs are canonicalized, derivatives are expanded, synthetic sources are injected.
4. Validation runs on the final merged result — not on individual fragments.

## Cross-file references

Individual files do not need to be self-contained. A source file can reference a category defined in `categories.yaml`, an artifact defined in `artifacts/`, or other feeds defined in other source files. The loader resolves all references after merging.

## Adding a new feed

Create a new `.yaml` file in the right category subdirectory under `sources/`. Add the category definition to `categories.yaml` if the category is new. Reload with SIGHUP or restart the daemon.

## Adding a new merge

Create a new `.yaml` file in `merges/`. Reference existing feed names in the `sources` and optional `exclude` lists. The referenced feeds can live anywhere in the catalog. Add `history:` only when the derived windows should be based on the merged output.

## Shared registries

| File | Purpose |
|------|---------|
| `runtime.yaml` | All daemon-level settings: concurrency, cadence, health thresholds, web URLs |
| `categories.yaml` | Public taxonomy: labels, descriptions, colors, sort order |
| `defaults.yaml` | Canonical ASN and GeoIP provider selection |
| `renames.yaml` | Backward-compatible name aliases |
| `deleted.yaml` | Names that are permanently removed and should not be reused |
| `critical_asn_context.yaml` | Secondary ASN-level context for blast-radius analysis |
