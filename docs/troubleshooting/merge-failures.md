# Merge Failures

You will learn why merges fail and how to fix them.

## How merges work

A merge feed composes its output from multiple parent feeds:

```
output = union(additive parents) - union(subtractive parents)
```

Additive parents are listed in the `sources` field. Subtractive parents are listed in the `exclude` field. All parents must be available and healthy for composition to succeed.

## Missing additive input

When an additive parent has no committed local feed body, the merge cannot compose.

**What you see:** The merge shows `download_failed` or `missing_input` in the admin UI.

**What it means:** The parent feed has never been downloaded or its local data was lost.

**How to fix:** Recheck the missing parent:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<parent-name>/recheck
```

Once the parent has data, the merge succeeds on its next scheduled run.

If all additive parents are missing or ineligible, the merge is operationally disabled. Fix the parents first.

## Missing subtractive input

When a subtractive parent (exclusion list) is missing, the merge fails on purpose.

**Why:** Publishing without the exclusion would broaden the merge output. The missing exclusions would let IPs through that the configuration intends to block.

This is a safety feature, not a bug.

**How to fix:** Recheck or re-enable the missing subtractive parent:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<exclusion-name>/recheck
```

## Health-excluded input

Parents with health class `archived` or `unmaintained` are excluded from merge composition.

**What you see:** The admin UI shows the excluded parent with its health class and exclusion reason in the merge detail view.

**How to fix:**

1. Check the parent's health in the admin UI
2. If the parent is `archived`, try a manual recheck to see if the upstream has recovered
3. If the parent is permanently dead, consider removing it from the merge configuration

## When a merge is stuck

A merge stays in a failed state when its dependencies are not met. Check all parent feeds:

```bash
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds | \
  jq '.[] | select(.name | startswith("firehol_")) | {name, health, last_status}'
```

Look for parents that are `unavailable`, `archived`, or showing download errors. Fix the parent feeds first — the merge recovers automatically on its next scheduled run.

## Forcing a merge run

After fixing parent feeds, trigger the merge immediately:

```bash
# Recheck the merge (recomposes from current parent data)
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<merge-name>/recheck
```

Or trigger all due work:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/run
```

## Checking merge composition

The admin API shows the current composition state for each merge:

```bash
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<merge-name> | \
  jq '.merge_included, .merge_excluded, .merge_health_excluded'
```

This shows which parents are currently included, which are subtracted, and which are excluded due to health.
