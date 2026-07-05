# Installation

You will learn how to build, install, and verify update-ipsets on a Linux server.

## Prerequisites

- Linux (amd64 or arm64)
- Go 1.26 or later
- pnpm (for building the embedded web UI)

## Build from source

Clone the repository and build:

```bash
git clone https://github.com/firehol/update-ipsets.git
cd update-ipsets
make build
```

This produces a single `update-ipsets` binary in the project root. The binary has the web UI embedded — no separate static files to deploy.

## Install

Run the installer:

```bash
./install.sh
```

The installer does these things in order:

1. Installs UI dependencies and builds the embedded web UI (`pnpm --dir ui install --frozen-lockfile` + `pnpm --dir ui build`)
2. Copies the fresh UI bundle into the embedded static directory
3. Builds the Go binary with the UI baked in
4. Creates the directory tree under `/opt/update-ipsets/`
5. Creates the `iplists` system group and user if they do not already exist
6. Installs the binary to `/opt/update-ipsets/bin/update-ipsets`
7. Deploys the feed catalog from `configs/firehol/` to `/opt/update-ipsets/etc/config/`
8. Copies Markdown templates from `configs/templates/markdown/` to `/opt/update-ipsets/etc/config/templates/markdown/`
9. Makes `bin/` and `etc/` owned by `root:iplists` with group-only access, and makes runtime directories writable by `iplists`
10. Installs the systemd unit at `/etc/systemd/system/update-ipsets.service`
11. Reloads systemd, restarts the service if it is active, starts it if it is enabled but inactive, or leaves it stopped if it is not enabled

## Installed service defaults

The generated systemd unit:

- runs as `User=iplists` and `Group=iplists`
- sets `UMask=0077` so generated artifacts are private to the service user
- serves the public listener on `127.0.0.1:18888`
- serves admin on `127.0.0.1:18889` with admin auth disabled
- moves the admin listener to the Tailscale IPv4 address when `tailscale ip -4` returns one
- sets `MemoryHigh=2.5G`, `MemoryMax=3G`, and `GOMEMLIMIT=2GiB`
- writes logs to the `iplists` journal namespace

The Tailscale address is detected when `install.sh` writes the unit. If
Tailscale is added later or the address changes, rerun the installer or set
`UPDATE_IPSETS_ADMIN_LISTEN_ARG` in a drop-in.

The Tailscale default is for hosts where tailnet access is the admin access
control layer. On a shared, untrusted, or internet-reachable network, override
the admin listener and set `UPDATE_IPSETS_ADMIN_AUTH_ARG=--admin-auth-mode=required`
with credentials in a protected drop-in.

### Custom install directory

Pass a different path as an argument only for manual or experimental layouts:

```bash
./install.sh /opt/custom-path
```

The installer copies the binary and catalog to that path and writes that path
into the generated systemd unit. For repeatable managed installs, use one stable
install directory so path overrides, backups, and service state stay together.

### Skip restart

Add `--no-restart` to install without restarting the running service:

```bash
./install.sh --no-restart
```

The new binary takes effect on the next manual restart.

## Configuration and template handling on reinstall

The installer compares the repository catalog (`configs/firehol/`) against the active config directory (`/opt/update-ipsets/etc/config/`). When the content changed, it:

- Creates a timestamped backup: `/opt/update-ipsets/etc/config.bak.YYYYMMDDHHMMSS`
- Deploys the fresh catalog from the repository

When the content is identical, the installer leaves the active config untouched. This avoids triggering unnecessary reprocessing.

Any local changes to the config directory survive reinstalls in the backup copy. Merge your customizations back after upgrading.

Markdown templates are handled separately. The installer copies repository
templates from `configs/templates/markdown/` into
`/opt/update-ipsets/etc/config/templates/markdown/`. If the installed templates
are identical, it leaves them untouched. If they differ, it overwrites matching
template files in place and does not create a separate template backup. Extra
local files under the template directory are not removed.

If you customize Markdown templates, keep a copy outside the installed template
directory or keep a local patch, then reapply it after reinstalling. Template
changes require a service restart; SIGHUP reloads the YAML catalog only.

## Verify the installation

Check the binary version:

```bash
/opt/update-ipsets/bin/update-ipsets version
```

Check the service status:

```bash
systemctl status update-ipsets
```

Inspect service logs:

```bash
journalctl --namespace=iplists -u update-ipsets -n 50
```

Test the health endpoint:

```bash
curl http://localhost:18888/healthz
```

A working installation returns `ok`.

## Next steps

- [Systemd setup](systemd-setup.md) — customize the service with drop-in overrides
- [TLS configuration](tls-configuration.md) — enable HTTPS
- [Filesystem layout](filesystem-layout.md) — understand the installed directory tree
