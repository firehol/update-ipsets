# Config Catalog Decomposition

## TL;DR

Purpose: make feed/catalog management maintainable for contributors and operators by replacing the monolithic authored catalog with a directory-based catalog, one file per feed.

Costa approved implementation and explicitly decided that backwards compatibility with the monolithic authored config is not required.

## Analysis

Verified facts:

- `configs/firehol.yaml` is the only committed catalog file today.
- `pkg/engine/runtime.go` resolves the default config path from `configs/firehol.yaml`, `configs/firehol.yml`, then `/etc/firehol/update-ipsets.*`.
- `install.sh` deploys `configs/firehol.yaml` to `/opt/update-ipsets/etc/config.yaml` and starts the daemon with `--config /opt/update-ipsets/etc/config.yaml`.
- `pkg/config.Load()` currently dispatches only by extension; a directory path is not accepted as the primary config.
- `pkg/config.LoadDirectory()` is non-recursive and validates/expands each file independently through `LoadYAML()`. That is wrong for a decomposed catalog because per-feed files depend on shared `categories`, `artifacts`, and cross-feed merge references.
- `Config.Merge()` merges maps but does not preserve `SourceOrder` or `ArtifactOrder`, which would make directory fragment order unstable.
- `LoadYAML()` currently does these steps in one function: read file, reject removed legacy blocks, unmarshal, normalize metadata, normalize health thresholds, expand history/merges, inject synthetic sources, normalize artifact-backed sources, canonicalize outputs, validate.
- The safe implementation is to split that pipeline into "load raw fragment" and "finalize merged config", then use the same finalization for both a single YAML file and a directory catalog.
- The current authored YAML top-level sections are `runtime`, `categories`, `artifacts`, `sources`, `infrastructure_asns`, `merges`, `renames`, and `deleted`.
- Existing tests directly load `configs/firehol.yaml`; they must load the new directory catalog instead.
- `specs/config.md`, `specs/files-layout.md`, `README.md`, and `AGENTS.md` still describe `configs/firehol.yaml` as the catalog source.

## Decisions

- Made by Costa on 2026-04-26: no backwards compatibility is needed for the old monolithic authored catalog.
- Therefore the primary distribution config can move to a directory-based layout without supporting `configs/firehol.yaml` as an alternate source.

## Plan

1. [x] Inspect current config model, loader, expander, validator, installer, tests, and catalog YAML shape.
2. [x] Define the new directory layout.
3. [x] Generate the existing catalog into the new layout.
4. [x] Update loader/install defaults to use the directory catalog as primary.
5. [x] Remove the monolithic authored catalog from the active source tree.
6. [x] Update specs and contributor/operator docs affected by config semantics.
7. [x] Add/adjust tests for directory catalog loading and validation.
8. [x] Run tests and install.
9. [ ] Commit after final review.

## Implied Decisions

- Keep runtime semantics unchanged unless the implementation proves a conflicting old assumption.
- Keep generated/expanded in-memory `Config.Sources` as the normalized model so the engine does not need broad changes.
- Prefer simple YAML fragments over inventing a new format.

## Testing Requirements

- [x] Config loader/validator tests: `go test ./pkg/config ./pkg/processor -count=1`
- [x] Full Go tests: `go test ./...`
- [x] UI/binary build and install: `./install.sh`
- [x] Runtime smoke test: `curl -fsS http://localhost:18888/healthz`
- [x] API smoke test: `curl -fsS http://localhost:18888/api/v1/status`

## Documentation Updates Required

- `specs/config.md`
- `specs/files-layout.md`
- `specs/feeds.md` if feed authoring semantics change there
- `README.md` install/config references
- contributor docs for adding feeds
