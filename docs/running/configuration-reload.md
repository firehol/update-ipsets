# Configuration Reload

You will learn how to reload the daemon configuration without restarting, what reloads, what does not, and how the pipeline handles in-flight work during a reload.

## Triggering a reload

Send `SIGHUP` to the daemon process. The daemon reads the updated YAML catalog and applies the new configuration.

```bash
systemctl kill -s HUP update-ipsets
```

If you run the daemon outside systemd, send `SIGHUP` to that specific process:

```bash
kill -HUP <update-ipsets-pid>
```

## What reloads

- YAML catalog: source definitions, merges, artifact parents, runtime settings, categories, supporting registries
- Derived feed configurations: history derivatives, merge expansions, artifact-backed children
- Effective enable/disable state, recalculated from the new catalog and the existing source/artifact enable marker files

The daemon re-reads the `--config` directory, re-validates everything, and swaps to the new configuration atomically.

## What does not reload

These require a full restart:

- Listen addresses (`--listen`, `--admin-listen`)
- TLS certificates and keys (`--tls-cert`, `--tls-key`)
- Log level (`--silent`, `--verbose`)
- Markdown templates under `templates/markdown/`

To change these, restart the service:

```bash
systemctl restart update-ipsets
```

## Invalid configuration handling

If the new configuration fails validation, the daemon rejects it and keeps the previous configuration active. The daemon logs the validation error with enough detail to identify the problem.

The daemon does not swap to a partial or broken configuration. Committed state, staged state, and queue ownership are never corrupted by a failed reload.

## Effect on running work

- In-flight downloads and processing batches complete using the configuration they started with.
- New work picked up after the reload uses the new configuration.
- No feed is left in a half-processed state by a reload boundary.

## When to reload versus restart

| Change | Reload | Restart |
|---|---|---|
| Add, remove, or edit a feed definition | yes | |
| Change runtime settings in the catalog | yes | |
| Change `--listen` or `--admin-listen` | | yes |
| Change TLS certificates | | yes |
| Change `--interval` | | yes |
| Change log level | | yes |
| Change Markdown templates | | yes |
| Fix a broken feed definition that blocked reload | | yes (or fix the config and reload again) |

## Checking reload status

After a reload, the daemon logs the outcome. Check the journal:

```bash
journalctl --namespace=iplists -u update-ipsets -n 50 --no-pager
```

The admin status endpoint also shows the current configuration state:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://127.0.0.1:18889/api/v1/admin/status
```

## See also

- [Daemon Reference](daemon-reference.md) — all flags and subcommands
- [Admin Authentication](admin-authentication.md) — accessing admin endpoints
