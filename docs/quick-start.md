# Quick Start

You will learn how to build update-ipsets from source, run it locally, and try its main features in under 5 minutes.

## Prerequisites

- Go 1.26 or later
- Git
- A terminal

## Build from source

Clone the repository and build the binary:

```bash
git clone https://github.com/firehol/update-ipsets.git
cd update-ipsets
make build
```

This produces the `update-ipsets` binary in the project root.

## Run the daemon

Start the daemon with the bundled catalog on a local port:

```bash
./update-ipsets daemon \
  --config configs/firehol \
  --listen :18888 \
  --admin-auth-mode=disabled \
  --allow-unauthenticated-admin
```

Flags explained:
- `--config configs/firehol` — use the bundled feed catalog
- `--listen :18888` — serve on port 18888
- `--admin-auth-mode=disabled --allow-unauthenticated-admin` — open admin for local testing (do not use in production)

## Open the interfaces

- **Public site:** [http://localhost:18888/](http://localhost:18888/)
- **Admin UI:** [http://localhost:18888/admin](http://localhost:18888/admin)

The public site shows the feed explorer, IP search, and comparisons. The admin UI shows download/processing queues and feed status.

## Try the CLI

Look up which feeds contain an IP address:

```bash
./update-ipsets query 1.2.3.4
```

Count unique IPs across CIDR ranges using the iprange subcommand:

```bash
printf "1.0.0.0/8\n2.0.0.0/8\n" | ./update-ipsets iprange --count-unique
```

Compose sets and test membership:

```bash
./update-ipsets query --set "firehol_level1 + firehol_level2" 1.2.3.4
```

## Next steps

- [Installation](installation/installation.md) — production deployment with systemd, TLS, and memory planning
- [Configuration Concepts](configuration/configuration-concepts.md) — how the YAML catalog works
- [Feed Families](feeds/feed-families.md) — the six feed families and when to use each
