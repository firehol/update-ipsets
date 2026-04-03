TL;DR
- Admin API already returns 344 feeds, but the admin UI shows only 326.
- Goal: admin UI must show all feeds, including geo, ASN, and infrastructure feeds.

Analysis
- Live admin API count is 344 via /api/v1/admin/feeds.
- Backend feed assembly in pkg/web/admin.go walks the configured/expanded sources and does not exclude geo/ASN/infra kinds.
- Existing backend tests confirm synthetic/hidden sources are exposed by the admin API.
- The likely bug is in the frontend count/filter logic in ui/src/components/admin/feeds-table.tsx.
- Follow-up evidence: the frontend bug is fixed, but the `Infra` kind still shows `0` because the backend only emits kind `infrastructure` for feeds with `use: [critical_infrastructure]`.
- Live config evidence: `/opt/update-ipsets/etc/config.yaml` currently has `0` such feeds, while it has `37` feeds in category `malware_infrastructure` and `5` in `provider_infrastructure`.
- Further analysis: the current admin filter model mixes orthogonal dimensions.
  - `health` currently includes `disabled`, `hidden`, and `historical` in `ui/src/components/admin/feeds-table.tsx`.
  - `kind` currently includes `infrastructure`.
  - There is no independent category filter row today.
  - There is no multi-select filter model today; both rows are single-select chips.
- Further analysis: the current filter counters are not faceted.
  - The table passes raw `feeds` into every count helper in `ui/src/components/admin/feeds-table.tsx`.
  - `computeHealthCounts(feeds)`, `computeKindCounts(feeds)`, `computeCategoryCounts(feeds)`, and `computeBooleanCounts(feeds, ...)` all count against the full feed list instead of "all other active filters except this axis".
  - This means selecting filters does not update the other rows the way operators expect from e-commerce-style faceted filtering.

Decisions
- User decision already made: admin must show all feeds; geo, ASN, and infra must not be hidden.
- Superseded decision: kind `infrastructure` is not used in admin.
- Decision made: the admin filter model is reworked into independent controls:
  - health
  - kind
  - category
  - hidden
  - disabled
- Decision made: `historical` is removed from `health`.
- Superseded decision: boolean controls do not use tri-state semantics.
- Decision made: boolean controls follow the same multi-select model as the other filters.
- Implication: for `hidden` and `disabled`, no chip selected means "show all", and selecting both boolean values is also equivalent to "show all".
- Pending decision: counter semantics for filter rows.
- Decision made: adopt faceted counts.
- Definition: each filter row's counters should be computed with all other active filters applied, but not the current row itself.
- Implication: selecting any filter updates the counters on the other rows immediately, while the current row keeps showing its own available values under the already-active external constraints.
- Implementation note: search text participates in the faceted result set, so counters reflect the currently searched subset too.
- Implication: `kind` returns to a strict operator feed-family grouping:
  - `source`
  - `merge`
  - `retention`
  - `asn`
  - `geolocation`
  - `bogon`
- Implication: sources with `critical_infrastructure` role and sources in infrastructure-related categories remain ordinary `source` feeds for admin-kind purposes.
- Implication: infrastructure-related feeds are still visible in the all-feeds table, but they are filtered by category, not by a dedicated `kind`.

Plan
- Inspect live API payload for hidden/kind distribution.
- Inspect React table filters/counts and remove the exclusion.
- Verify in the built UI or by test/build.
- Rework admin filter semantics:
  - make each axis independent
  - add dynamic category control
  - remove `infra` from kind
  - remove `historical` from health
  - keep hidden/disabled as independent multi-select controls
- Restore backend kind classification so infrastructure-related feeds are no longer exposed as kind `infrastructure` at all.

Implied decisions
- Hidden/internal classification may remain as metadata or badges, but it must not remove feeds from the admin list by default.

Testing requirements
- Verify admin API still returns 344.
- Verify frontend count uses all feeds.
- Run targeted frontend/backend tests if available, plus a build if needed.
- Verify filter UX remains responsive in both directions:
  - applying filters (shrinking the result set)
  - clearing/unfiltering (growing the result set back to the full table)
- Verify the per-row clear action stays visually stable and does not shift the chip rows.
- Verify long values in the `State` column wrap inside the table instead of forcing horizontal overflow.

Documentation updates required
- Update the admin UI spec to define the independent filter axes and boolean multi-select behavior.
- Remove the previous temporary spec note that treated `Infra` as an operator kind grouping.

Follow-up analysis
- New operator report: filtering is fast, but unfiltering takes about 1s.
- Code evidence in `ui/src/components/admin/feeds-table.tsx`:
  - every filter change recomputes the filtered table plus five separate faceted count maps
  - clearing a filter usually grows the visible row count sharply, so React must mount many more table rows and cells at once
  - the current per-row `clear` control appears before the chips, so it shifts the row layout when it appears/disappears
- Working theory to verify/fix:
  - the latency is dominated by rendering the larger table, not by the boolean/count math itself
  - the clear control should move to the end of the row to avoid layout churn
- User decision made:
  - tooltip content in the admin table should be lazy-mounted
- User decision made:
  - remove all tooltips from the admin feed list rows entirely
- User decision made:
  - the `State` column should wrap long content instead of truncating or pushing the table off-screen
- Additional evidence:
  - the admin feed table row contains many tooltip triggers (`HoverTip`) in `ui/src/components/admin/feeds-table.tsx`
  - some remaining browser-native tooltips still exist and should be removed from the admin experience:
    - `ui/src/components/admin/feeds-table.tsx` uses `title={feed.last_error}`
    - `ui/src/components/admin/feeds-table.tsx` uses `title={scheduleState}`
    - `ui/src/components/category-badge.tsx` uses `title={meta?.description}`
- Implementation direction:
  - remove row-level `HoverTip` usage from the admin feed table entirely
  - keep non-row admin help such as header hints unchanged unless requested separately
  - remove browser `title=` tooltips from the admin table path
