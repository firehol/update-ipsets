# Admin UI Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

The admin UI is the operator control surface of the product.

It exists to let an operator:

- understand current runtime activity
- inspect feeds and artifact parents
- trigger the supported manual operations
- identify actionable local problems

The admin UI surfaces two real operational subsystems:

- the downloader
- the processing engine

Their detailed contracts live in:

- [downloader.md](downloader.md)
- [processing-engine.md](processing-engine.md)

## Access model

The admin surface MUST be a distinct operator surface.

The product MAY choose any transport or authentication implementation, but the
operator contract is:

- the admin surface is separate from the public site semantically
- the implementation MAY serve public and admin from one listener or from
  separate listeners
- unauthenticated users MUST NOT access operator controls or operator-only data
  unless the operator has explicitly selected an unsafe no-auth mode

For the current web product, this requirement applies to the admin HTML routes
as well as the authenticated JSON APIs:

- `/admin`
- `/admin/*`
- `/api/v1/admin/*`

The admin SPA shell itself is part of the operator surface. It MUST NOT be
served to unauthenticated users when admin credentials are configured.

If the chosen admin-auth mechanism is not configured correctly, the admin
surface MUST fail closed. Misconfiguration MUST NOT make the admin HTML routes
or admin APIs public.

### Listener topology

The product MUST support both of these topologies:

- shared mode:
  - public and admin routes are served on the same listener
- split mode:
  - public routes are served on the public listener
  - admin HTML routes and `/api/v1/admin/*` are served on a separate admin
    listener

Rules:

- split mode MUST be opt-in
- when split mode is enabled, admin routes MUST NOT be exposed on the public
  listener
- in split mode, requests for `/admin`, `/admin/*`, and `/api/v1/admin/*` on
  the public listener MUST fail as not found rather than serving admin content

### Authentication modes

The product MUST support an explicit admin authentication mode equivalent to:

- `required`
- `disabled`

Rules:

- `required` is the safe/default mode
- `required` mode continues to use the configured admin credentials and MUST
  fail closed if they are not configured correctly
- `disabled` is an explicitly unsafe operator-selected mode intended for local
  development or otherwise operator-managed trusted environments
- selecting `disabled` MUST require a second explicit unsafe acknowledgment knob
- the product MUST NOT infer `disabled` from missing credentials
- the product MUST NOT infer safety from the bind address alone

### Public links from the admin UI

The admin UI may link to public website pages such as feed-detail routes.

Rules:

- those links MUST use the configured public website base URL when provided
- true split-listener deployments MUST NOT rely on same-origin relative links
  for admin-to-public navigation
- when split mode is enabled, the public website base URL MUST be configured so
  admin-to-public links resolve to the public site rather than the admin
  listener

## Main admin surfaces

The admin product MUST provide at least these surfaces:

### 1. Runtime status

Shows:

- overall service state
- current pipeline state
- active work
- queue backlog

### 2. Feed inventory

A full table of feeds remains the main operator inventory.

This table MUST NOT be replaced by transient queue panels.

### 3. Artifact inventory

Artifact parents MUST be visible and manageable separately from feed rows.

### 4. Schedule visibility

Operators MUST be able to inspect when items are expected to run and why.

### 5. Integrity

Operators MUST be able to inspect settled integrity findings and request
recovery.

The entity-integrity surface MUST also offer an explicit operator action to
queue a full rebuild of all country and ASN artifacts from scratch as visible
background work.

Integrity finding tables MUST be bounded viewports with their own scrollbars
when findings are numerous. A large integrity or entity-integrity result set
MUST NOT monopolize the whole admin page or push unrelated operator panels out
of practical reach.

## Operator workspace behavior

The admin UI SHOULD behave as an operator workspace rather than a public
website page.

Rules:

- feed-table filters, search terms, sort state, and selected feed details
  SHOULD be represented in URL search parameters so browser back/forward,
  refresh, and copied URLs preserve operator context
- selecting a feed from an admin list SHOULD preserve the surrounding list
  context; a drawer or master-detail view is preferred over a centered modal
  for feed inspection
- command-palette navigation MAY be provided for fast jumps to important
  panels and feed records
- command-palette or keyboard-driven actions MUST have visible UI equivalents
  and MUST NOT hide required operator actions behind shortcuts only
- long-running or high-cost actions SHOULD require an explicit confirmation
  step that describes the broad operational effect

## Queue model in the admin UI

The admin top area MUST expose only these four live lists:

1. waiting to be downloaded
2. being downloaded now
3. waiting to be processed
4. being processed now

For the current web admin UI, each of these live-list viewports MUST reserve a
stable fixed height sized for roughly four visible feed rows and then scroll
for overflow. Auto-refresh MUST NOT cause the rest of the page to jump
vertically just because the queue lengths changed.

Normal page panels and tables MUST NOT trap page-wheel scrolling merely because
the pointer is hovering them. Vertical self-scrolling MUST be used only for
intentional bounded viewports such as these live lists or explicit modal
content; ordinary page tables SHOULD prefer horizontal overflow only.

These lists MUST correspond to the real downloader-loop and processing-loop
state. The admin UI MUST NOT present them as if one combined internal
"scheduler" owned both queue families.

Any queue-item status or subtitle shown in these live lists MUST use the same
operator-facing status meaning contract as the feed and artifact inventories.
The live queue views MUST NOT render raw cache/downloader implementation codes
such as `parse_failed` directly to operators.

Downloader-stage lists MAY include:

- normal feeds
- artifact parents
- provider datasets

Processing lists are feed-only operator views.

`waiting to be processed` MUST contain only:

- feeds whose downloader-stage outcome has already admitted them for processing
- feeds admitted by a downloader-originated provider-refresh reprocess wave
- feeds restored from restart recovery of an already durable staged or
  processing feed body
- feeds admitted by integrity-triggered local engine/output repair reprocess
- feeds explicitly queued for reprocess from committed local feed-body state

No other ordinary runtime condition may place a feed there directly.

The UI MUST NOT expose a separate pseudo-batch or pseudo-queue that has no
operator meaning.

`being processed now` MUST show more than the feed name and phase when the
backend reports active operation progress. For each active feed, the admin UI
SHOULD render:

- the current operation/stage label
- the declared unit of work
- completed work and total work
- completion percentage when the operation is bounded
- processing rate in the same unit per second
- a stable progress bar or equivalent compact visual indicator

The progress unit and rate MUST come from the backend status contract or be
derived directly from backend-provided `current`, `total`, `unit`, `elapsed_ms`,
and `rate_per_second` fields. The UI MUST NOT infer semantic units from feed
names or operation-name substrings.

## Background-work visibility

The daemon MUST NOT perform invisible background work.

If the daemon runs operator-relevant work outside the four live queue lists,
the admin API/UI MUST expose it explicitly as background work rather than
pretending it is part of one of the queue families.

Examples include:

- startup artifact rebuilds
- config-reload artifact rebuilds
- health-transition refresh work
- feed-update country/ASN entity refresh work
- operator-requested full country/ASN rebuilds

For such tasks, the admin status surface SHOULD expose, at minimum:

- task name
- trigger/source of the task
- current stage
- started-at time
- progress when meaningful

These background-work entries are not a fifth queue. They are a separate
operator-facing status block for daemon activity that does not belong to the
real downloader or processing queues.

The admin UI SHOULD render the background-work block even when there is no
active background work, so the operator can distinguish "idle" from "feature
missing" or "status failed to load".

Repeated background requests for the same logical maintenance target SHOULD be
coalesced before they become visible tasks. For example, ordinary feed-update
and health-transition country/ASN entity refresh work MUST deduplicate feed
names while a previous refresh is queued or running, instead of showing and
executing one serial background task per scheduler tick or processing batch.

Background tasks that mutate country/ASN entity artifacts MUST serialize their
publication even when the configured background worker limit is greater than
one. The worker limit controls how much background work may be admitted, but it
MUST NOT allow two entity writers to publish overlapping private or public
country/ASN artifacts at the same time.

## Resource telemetry visibility

The admin status surface MUST expose enough daemon-lifetime telemetry to
identify resource waste without reading logs or guessing from wall-clock time.

At minimum, the admin API SHOULD expose:

- monotonic operation counters and timing totals for major CPU-affecting work
- monotonic byte/count counters for network, file, JSON, mmap, sidecar, and
  artifact work where those operations materially affect resource use
- process CPU user/system/total seconds
- process memory size and resident set snapshots
- process read/write bytes and read/write syscall snapshots when the platform
  exposes them
- current background-maintenance worker limit and active worker count

These values MUST be snapshot-diff friendly. An operator or automated test
SHOULD be able to sample the admin status endpoint twice, compare counter
deltas against elapsed time and process CPU deltas, and identify which
operations moved the most.

Admin polling endpoints are themselves part of the operational profile. Any
endpoint polled by the admin UI SHOULD expose request count, response bytes, and
build/write timings. Those endpoints MUST reuse already-built snapshots inside a
single request and MUST NOT perform per-row full-cache snapshots or other
O(rows * total-state) work.

Telemetry names SHOULD distinguish skipped work from real work. For example,
pairwise comparison counters SHOULD separate candidate pairs, overlap checks,
and skip reasons rather than collapsing them into one generic bucket.

## Meaning of "being processed now"

`being processed now` means:

- the items that belong to the current active processing batch
- and are actively being executed now

This panel MUST NOT show terminal states such as:

- failed
- updated
- skipped

Those are historical results, not active work.

## Waiting-to-be-processed contract

The waiting-to-be-processed view MUST tell operators how long an item has been
waiting.

Reason text MAY also be shown, but queue age MUST be visible because it has
direct operational meaning.

Queue age means total time since the item first entered the waiting-to-be-
processed queue for the current staged body.

If processing fails and the same staged body is requeued, the operator-visible
queue age MUST continue from the original first admission; it MUST NOT reset on
that retry.

## Feed inventory contract

The all-feeds table MUST remain stable and comprehensive.

The table MUST remain accessible while preserving operator density:

- keyboard activation MUST be provided by native links or buttons
- rows MUST NOT be exposed as focusable controls when they contain nested
  focusable actions such as public-page links
- every table header, including icon/action columns, MUST have a screen-reader
  name

The filter controls above this table MUST model independent operator concerns.
They MUST NOT collapse orthogonal dimensions into one row.

At minimum the table controls MUST expose these independent filters:

- feed health
- feed kind
- category
- hidden state
- disabled state

The health filter MUST include only health classes:

- healthy
- delayed
- risky
- unavailable
- empty
- unmaintained
- archived

The kind filter MUST include only these operator-facing kinds:

- source
- merge
- retention
- asn
- geolocation
- bogon

These admin kinds are operator-facing projections of the broader product
taxonomy:

- plain feeds map to `source`
- history derivatives map to `retention`
- merges map to `merge`
- sources surfaced primarily for ASN enrichment map to `asn`
- sources surfaced primarily for geolocation enrichment map to `geolocation`
- sources surfaced primarily for bogon enrichment map to `bogon`

Artifact parents are managed in the separate artifact inventory, not in the
feed-kind filter.
Infrastructure-related feeds are filtered by category, not by a dedicated kind.

Where multiple values make sense, filter controls SHOULD support multi-select.
Boolean filters such as hidden/disabled SHOULD follow the same selection model:
when no value is selected, the table shows all feeds for that axis.

Filter counters SHOULD behave as faceted counts:

- selecting values on one axis SHOULD update the counters on the other axes
- a row's own counters SHOULD be computed with all other active filters applied,
  but not that row itself

For each feed, the operator SHOULD be able to inspect at least:

- enable state
- health
- hidden state
- disabled state
- current status
- last check
- last observed change
- last successful local publication
- next scheduled action or trigger reason
- failure streak / failure count
- last run reason
- last processing duration
- for merge feeds: current included inputs, configured subtracted inputs,
  health-excluded inputs, and exclusion reasons

The admin UI MUST describe the real operational meaning of these values, not
just raw stored state.

Any internal feed state or flag that affects scheduling, publication,
integrity, or operator actions MUST be exposed through the admin API/UI and be
inspectable in the feed detail/modal surface. Operators MUST NOT be blind to an
internal feed state the product is acting on.

The authenticated admin API for feed rows and artifact-parent rows MUST expose
operator-facing status meaning directly instead of forcing the UI to decode raw
cache/downloader implementation codes. At minimum, those machine-readable row
shapes MUST include:

- an operator-facing label for the latest settled local status
- a machine-readable problem classification when the latest settled local
  actionable fault is known to be downloader-stage or processing-stage

Recommended field names are:

- `last_status_label`
- `last_problem_class`

`last_problem_class` MUST distinguish at least:

- `downloader`
- `processing`

Settled integrity state remains a separate contract exposed by the integrity
surface and MUST NOT be flattened into ordinary last-status fields.

## Artifact inventory contract

For each artifact parent, the operator SHOULD be able to inspect at least:

- enable state
- last check
- last change
- next scheduled check
- family/type
- child feed relationships

The operator MUST be able to enable and disable artifact parents separately from
feed rows.

## Supported operator actions

The admin UI MUST support these operations:

### Batch-level actions

- run due work now
- force broad reprocessing where supported
- request integrity recovery

### Feed-level actions

- enable
- disable
- recheck
- reprocess

Provider databases that are modeled as source rows use the same feed-level
action surface as other feeds.

### Artifact-level actions

- enable
- disable
- recheck the parent artifact

## Admin API contract

The admin surface MUST expose stable authenticated machine-readable endpoints
equivalent to:

- `GET /api/v1/admin/status`
- `GET /api/v1/admin/feeds`
- `GET /api/v1/admin/feeds/{name}`
- `GET /api/v1/admin/feeds/{name}/manifest`
- `POST /api/v1/admin/feeds/{name}/recheck`
- `POST /api/v1/admin/feeds/{name}/reprocess`
- `POST /api/v1/admin/feeds/{name}/enable`
- `POST /api/v1/admin/feeds/{name}/disable`
- `GET /api/v1/admin/artifacts`
- `GET /api/v1/admin/artifacts/{name}` (returns artifact JSON detail)
- `POST /api/v1/admin/artifacts/{name}/recheck`
- `POST /api/v1/admin/artifacts/{name}/enable`
- `POST /api/v1/admin/artifacts/{name}/disable`

  Note: artifact actions are dispatched via sub-router from
  `POST /api/v1/admin/artifacts/` rather than individual route registrations.
  The GET on `/artifacts/{name}` returns a single artifact's detail JSON when
  the request method is GET and action is empty.
- `GET /api/v1/admin/schedule`
- `GET /api/v1/admin/integrity`
- `GET /api/v1/admin/integrity/entities`
- `POST /api/v1/admin/integrity/entities/rebuild`
- `POST /api/v1/admin/integrity/reprocess`
- `POST /api/v1/admin/run`

Rules:

- admin read endpoints are GET/HEAD surfaces; admin action endpoints are POST
  surfaces
- unsupported methods for known admin routes MUST return `405 Method Not
  Allowed` with an `Allow` header instead of reaching the opposite read/action
  handler
- feed-level `reprocess` MUST fail clearly when neither staged nor committed
  local input exists
- child-feed `recheck` MUST resolve to the correct artifact parent when the
  child lacks local materialized input
- archived feeds use the same feed-level actions as other feeds; there is no
  separate archived-only `check health` action
- feed-level `recheck` on an archived feed is an explicit operator-authorized
  refresh and MAY move the feed out of `archived` if it succeeds
- global `run due` and global `reprocess` are distinct operations and MUST NOT
  be conflated
- the admin API MUST NOT expose a separate per-feed `run` operation distinct
  from feed-level `recheck` / `reprocess`
- unmapped authenticated admin API paths MUST fail as API requests and MUST
  NOT return the admin SPA shell
- the authenticated admin API is part of the operator contract, not an internal
  UI-only convenience

## Action semantics

### Recheck

Recheck means:

- fetch now when upstream fetching is meaningful
- queue processing even if the fetched content did not change

### Reprocess

Reprocess means:

- do not fetch
- rerun processing from existing local input

`reprocess` is the normal direct engine-entry operation for feed rows.
The only non-admin automatic exception is integrity-triggered local engine/
output repair when the product already has enough committed or staged local
feed-body state.

There is no third feed-level action between `recheck` and `reprocess`.

If a product build exposes any UI label such as "run now" for a feed row, that
label MUST map unambiguously to either `recheck` or `reprocess`; it MUST NOT
introduce a separate undocumented operation.

Ordinary automatic runtime work must enter processing from downloader
admission, not by bypassing the downloader queue model.

### Integrity recovery

Integrity recovery means:

- inspect settled findings
- split them into the correct recovery class
- queue the needed repair work

The operator-facing integrity surface MUST show, per finding:

- the settled integrity reason
- malformed outputs separately from missing/stale outputs when present
- the actual recovery class (`recheck` or `reprocess`)
- the actual queued target(s)

The UI MUST NOT expose a misleading one-size-fits-all per-row action when the
real recovery plan for that finding targets a parent feed or artifact recheck.

For compatibility, the recovery endpoint MAY retain a legacy path name such as
`/api/v1/admin/integrity/reprocess`, but its semantic contract remains split
integrity recovery and its machine-readable response SHOULD distinguish queued
`recheck` targets from queued `reprocess` targets.

## Severe runtime faults

The admin product MUST surface severe runtime faults separately from normal
health or freshness status.

At minimum, operators MUST be able to tell whether the latest actionable
problem for a feed or runtime item is:

- a downloader-stage failure
- an integrity/recovery problem
- a severe processing-stage exception

This visibility MAY be provided in feed rows, detail panels, integrity views,
or another operator-focused surface.

It MUST NOT require reading daemon logs to distinguish these classes.

The operator-facing recovery classes are:

- `recheck`
- `reprocess`

## Integrity panel contract

The integrity panel MUST reflect settled integrity state.

The integrity panel MUST provide an operator-visible control, default off, to
include archived feeds in integrity reporting and recovery.

Changing this archived-inclusion control MUST invalidate the currently displayed
integrity result and force a fresh integrity pass for the newly selected scope.

Recovery actions MUST remain disabled until that scoped integrity pass
completes.

Once the scoped pass completes, recovery MUST apply only to the findings shown
by that scoped result.

If integrity work is currently running or blocked by current pipeline activity:

- the UI MUST communicate that state clearly
- and MUST clear it once the activity settles

The operator MUST NOT be left with a permanent stale "waiting" message after the
system has finished.

When the panel lists findings, it MUST describe the recovery that will happen.
It MUST NOT imply that every finding maps to direct feed-level reprocess.

### Entity-integrity panel

Entity-reference artifacts have a separate integrity surface from feed-output
integrity.

The admin UI MUST expose a separate read-only entity-integrity panel for
country/ASN artifacts.

That panel MUST show:

- clean / issues / in-progress state
- missing, stale, or malformed entity artifacts
- the affected scope (`feed`, `country`, `asn`, `index`, or `global`)
- the affected subject
- the repair action the daemon will take automatically when relevant

The current admin contract does not require a manual recovery button for entity
integrity. Operator visibility is mandatory; operator-triggered recovery is
optional.

## Schedule display rules

The admin UI MUST present schedule meaning, not raw timestamps only.

Examples:

- input-triggered derivatives SHOULD be described as input-triggered
- cadence-driven merges SHOULD be shown as cadence-driven
- values retained for stable ordering MAY remain internal as long as operator
  meaning is clear

## Design intent

The admin UI is an operations tool, not a marketing surface.

It MUST optimize for:

- clarity
- actionability
- stable meaning
- low surprise

It SHOULD avoid:

- decorative metrics without operator value
- duplicate surfaces that show the same thing with different words
- exposing internal bookkeeping that has no operational meaning
