# query Command

You will learn how to look up which feed lists contain a given IP and how to compose sets on the command line.

## Basic IP lookup

Find which lists contain an IP address:

```bash
update-ipsets query 192.0.2.1
```

Output lists every feed that includes this IP:

```
firehol_level1
firehol_level2
tor_exits
```

Add `--verbose` to see feed metadata alongside results. Add `--details` to show match context.

## Compose sets

Build a composed set from multiple feeds using `+` (include) and `-` (exclude):

```bash
update-ipsets query --set "firehol_level1 + firehol_level2 - tor_exits"
```

This dumps the resulting IP set to stdout. The composition expression follows these rules:

- `+` adds a feed (union)
- `-` removes a feed (exclusion)
- Feed names match the names in the configuration catalog
- The expression is evaluated left to right: additions first, then exclusions

## Test an IP in a composed set

Combine `--set` with an IP argument to test membership:

```bash
update-ipsets query --set "firehol_level1 + firehol_level2 - tor_exits" 192.0.2.1
```

Exit code 0 means the IP is in the set. Exit code 1 means it is not. Use `--silent` to suppress text output and rely only on the exit code.

## Output format

Control the output format of composed sets:

```bash
# CIDR notation (default)
update-ipsets query --set "firehol_level1" --format cidr

# Range notation
update-ipsets query --set "firehol_level1" --format range

# One IP per line
update-ipsets query --set "firehol_level1" --format single
```

## Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to the configuration catalog |
| `--set <expr>` | Composition expression with `+` and `-` |
| `--ip <addr>` | IP address to look up (positional argument also works) |
| `--format <fmt>` | Output format: `cidr`, `range`, or `single` |
| `--silent` | Suppress text output; use exit code only |
| `--verbose` | Show additional metadata and match details |

## Specifying the config path

By default, `query` looks for the config catalog in the same locations as the daemon. Override with `--config`:

```bash
update-ipsets query --config /opt/update-ipsets/etc/config 192.0.2.1
```

## Scripting examples

Check if an IP is blocked and exit accordingly:

```bash
if update-ipsets query --set "firehol_level1 + firehol_level2" --silent 192.0.2.1; then
    echo "BLOCKED"
else
    echo "ALLOWED"
fi
```

Count unique IPs in a composed set:

```bash
update-ipsets query --set "firehol_level1 + firehol_level2" --format single | wc -l
```

Dump a composed set to a file:

```bash
update-ipsets query --set "firehol_level1 + firehol_level2 - tor_exits" > filtered.ipset
```
