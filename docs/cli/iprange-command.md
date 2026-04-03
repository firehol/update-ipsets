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
- **@directory:** Read all `.ipset` and `.netset` files in a directory

Use `-` to read from stdin.

## Output formats

Control output format with flags:

- `--cidr` — output as CIDR (e.g., `192.0.2.0/24`)
- `--range` — output as ranges (e.g., `192.0.2.0-192.0.2.255`)
- `--single` — output one IP per line, expanding all ranges
- `--binary` — output in FileSet binary format

Default is CIDR.

## Operations

### Compare two sets

Show differences between two IP lists:

```bash
update-ipsets iprange --compare set1.ipset set2.ipset
```

Output shows added and removed ranges.

### Diff

Show only the changes (added IPs on the left, removed on the right):

```bash
update-ipsets iprange --diff set1.ipset set2.ipset
```

### Intersect

Keep only IPs present in both sets:

```bash
update-ipsets iprange --intersect set1.ipset set2.ipset
```

### Exclude

Remove IPs in the second set from the first:

```bash
update-ipsets iprange --exclude set1.ipset set2.ipset
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

Count across all files in a directory:

```bash
update-ipsets iprange --count-unique @/opt/update-ipsets/data/lib/
```

## Reduction

Use these flags when you need a representative sample instead of the full set:

- `--ipset-reduce <factor>` — reduce the set to approximately 1/N of its size
- `--reduce-factor <pct>` — keep approximately N% of the original ranges

Example — keep about 10% of ranges:

```bash
update-ipsets iprange --reduce-factor 10 --combine large1.ipset large2.ipset
```

## Reading from stdin

Pipe data in:

```bash
curl -s https://example.com/blocklist.txt | update-ipsets iprange --combine -
```

## Combining with the query command

Use iprange to prepare sets, then query them:

```bash
update-ipsets iprange --combine set1.ipset set2.ipset > merged.ipset
update-ipsets query --set "merged" 192.0.2.1
```
