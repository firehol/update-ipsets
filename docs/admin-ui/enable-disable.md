# Enable and Disable

You will learn how to enable and disable feeds and artifact parents, what happens to dependent feeds, and how enable state persists across restarts.

## Feeds and artifact parents are independent

You enable and disable feeds and artifact parents separately. Disabling a feed does not disable its artifact parent. Disabling an artifact parent operationally disables its children.

## Disabled feeds

When you disable a feed:

- The scheduler removes it from the download queue.
- No new downloads or compositions happen.
- No processing happens.
- Existing published artifacts remain in place.
- The feed still appears in the admin UI with its disabled state visible.

## Effect on merges

Merges compose from their enabled inputs. Disabling an input changes merge behavior:

| Input type | Effect of disabling |
|---|---|
| **Additive input disabled** | The input is excluded from composition. If no additive inputs remain eligible, the merge is operationally disabled. |
| **Subtractive input disabled** | The merge **fails composition**. This is a safety behavior — it prevents the published set from silently broadening. |

A subtractive input that is disabled, archived, unmaintained, or missing causes the merge to fail rather than producing a broader set than intended.

Inputs that are `unmaintained` or `archived` are also excluded from merge composition, same as disabled inputs.

## Artifact parents and children

An artifact-backed child feed is operationally enabled only when both conditions are true:

- The child itself is enabled.
- The artifact parent is enabled.

Disabling the parent disables all its children operationally, even if the children remain individually marked as enabled. Children cannot control whether the parent is enabled.

## History derivatives

A history derivative follows its parent. If the parent is disabled, the derivative is operationally disabled.

## Enable all

The `--enable-all` flag enables all known feeds at startup. This is useful for initial setup or when you want the daemon to manage every feed in the catalog.

```bash
update-ipsets daemon --enable-all
```

## Persistence

Enable state persists across daemon restarts via marker files on disk. When the daemon restarts, it reads the markers and restores each feed to its last known enable state.

You do not need to re-enable feeds after a restart.

## Provider databases

Provider databases follow the same enable/disable semantics as normal feeds.

When a provider database is disabled:

- The downloader stops refreshing it.
- The processing engine stops using newly refreshed data.
- Previously published artifacts from older provider data remain authoritative until later publication replaces them.

## See also

- [Feed Inventory](feed-inventory.md) — viewing and filtering feeds by disabled state
- [Artifact Inventory](artifact-inventory.md) — managing artifact parents
- [Health Classes](../pipeline/health-classes.md) — how health interacts with merge composition
