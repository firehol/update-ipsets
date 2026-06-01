# iprange Command

You will learn how to use the standalone iprange-compatible mode to compare, merge, and manipulate IP sets from the command line.

## Running iprange mode

```bash
update-ipsets iprange [options] [file ...]
```

This mode is a standalone IP set tool. It reads IP lists, performs set operations, and writes results. It does not require the daemon or a configuration file.

## Input formats

The tool accepts these input forms:

- **CIDR notation:** `192.0.2.0/24`
- **Range notation:** `192.0.2.0-192.0.2.255`
- **Single IPs:** `192.0.2.1`
- **Binary files:** FileSet format produced by the daemon
- **@filelist:** Read file paths from a text file, one per line
- **@directory:** Read all regular files in a directory, sorted by name

Use `-` to read from stdin.

## Output formats

Control output format with flags:

- default — output as CIDR (e.g., `192.0.2.0/24`)
- `--print-ranges` — output as ranges (e.g., `192.0.2.0-192.0.2.255`)
- `--print-single-ips` — output one IP per line, expanding all ranges
- `--print-binary` — output in FileSet binary format

Default is CIDR.

Use `-6` or `--ipv6` for IPv6 input. IPv4 is the default.

## Operations

### Compare two sets

Show differences between two IP lists:

```bash
update-ipsets iprange --compare set1.ipset set2.ipset
```

Output is CSV summary data for each compared pair: names, entry counts, unique IP counts, combined IPs, and common IPs.

### Diff

Show only the changes (added IPs on the left, removed on the right):

```bash
update-ipsets iprange set1.ipset --diff set2.ipset
```

`--diff` exits with code `1` when differences are found and `0` when the sets
are identical. It still prints the differing ranges unless `--quiet` is set.

### Intersect

Keep only IPs present in both sets:

```bash
update-ipsets iprange --intersect set1.ipset set2.ipset
```

### Exclude

Remove IPs in the second set from the first:

```bash
update-ipsets iprange set1.ipset --exclude-next set2.ipset
```

### Combine (union)

Merge all IPs from all input files:

```bash
update-ipsets iprange --combine set1.ipset set2.ipset set3.ipset
```

### Merge

Combine with automatic deduplication and range optimization:

```bash
update-ipsets iprange --merge set1.ipset set2.ipset
```

### Count unique IPs

Count the number of unique IP addresses across all inputs:

```bash
update-ipsets iprange --count-unique set1.ipset set2.ipset
```

Count across a directory that contains only IP range input files:

```bash
update-ipsets iprange --count-unique @/opt/update-ipsets/data/
```

## Reduction

Use these flags when you need a representative sample instead of the full set:

- `--ipset-reduce <pct>` / `--reduce-factor <pct>` — aliases. Allow up to
  `pct` percent more entries while collapsing output to coarser prefixes.
- `--ipset-reduce-entries <count>` / `--reduce-entries <count>` — set the
  minimum entry budget used by reduction.

Example — allow up to 10% entry growth while reducing prefix detail:

```bash
update-ipsets iprange --reduce-factor 10 --combine large1.ipset large2.ipset
```

## Supported flags

| Flag | Meaning |
|---|---|
| `-4`, `--ipv4` | IPv4 mode. This is the default. |
| `-6`, `--ipv6` | IPv6 mode. |
| `--min-prefix <prefix>` | Do not print prefixes smaller than this value. |
| `--prefixes <list>` | Restrict printed prefix sizes to a comma- or space-separated list. |
| `--default-prefix <prefix>`, `-p <prefix>` | Prefix length for bare IP input. |
| `--ipset-reduce <pct>`, `--reduce-factor <pct>` | Enable prefix reduction with the given allowed entry-growth percentage. |
| `--ipset-reduce-entries <count>`, `--reduce-entries <count>` | Minimum entry budget for reduction. |
| `--optimize`, `--combine`, `--merge`, `--union`, `--union-all`, `-J` | Combine all input sets. |
| `--common`, `--intersect`, `--intersect-all` | Keep only addresses common to all input sets. |
| `--exclude-next`, `--except`, `--complement-next`, `--complement` | Treat following inputs as exclusions from the earlier inputs. |
| `--diff`, `--diff-next` | Print differences between the earlier inputs and following inputs. |
| `--compare`, `--compare-first`, `--compare-next` | Print comparison summaries. |
| `--count-unique`, `-C` | Count unique addresses after merging inputs. |
| `--count-unique-all` | Count unique addresses per input set. |
| `--print-ranges`, `-j` | Print ranges instead of CIDR prefixes. |
| `--print-binary` | Print FileSet binary output. |
| `--print-single-ips`, `-1` | Print one IP per line. |
| `--print-prefix <text>` | Prefix every printed IP and network entry. |
| `--print-prefix-ips <text>` | Prefix only printed single-IP entries. |
| `--print-prefix-nets <text>` | Prefix only printed network/range entries. |
| `--print-suffix <text>` | Suffix every printed IP and network entry. |
| `--print-suffix-ips <text>` | Suffix only printed single-IP entries. |
| `--print-suffix-nets <text>` | Suffix only printed network/range entries. |
| `--header` | Include a CSV header where the selected output mode supports one. |
| `--quiet` | Suppress non-result output where supported. |
| `--dont-fix-network` | Preserve CIDR host bits instead of normalizing them to the network address. |
| `--dns-threads <count>` | Number of resolver workers for hostname input. |
| `--dns-silent`, `--dns-progress` | Accepted for compatibility. |
| `--has-compare`, `--has-reduce` | Print feature-detection output and exit. |
| `--has-filelist-loading`, `--has-directory-loading` | Print feature-detection output and exit. |
| `--version` | Print iprange compatibility version output and exit. |
| `--help`, `-h` | Print built-in usage and exit. |

Use `as <alias>` after an input path to set the name used in compare output.

## Reading from stdin

Pipe data in:

```bash
curl -s https://example.com/blocklist.txt | update-ipsets iprange --combine -
```

## Combining with the query command

Use `iprange` for local files. Use `query --set` for feed names from the configured catalog:

```bash
update-ipsets iprange --combine set1.ipset set2.ipset > merged.ipset
update-ipsets query --set "firehol_level1 + firehol_level2" 192.0.2.1
```
