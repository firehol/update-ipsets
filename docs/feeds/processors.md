# Processor Reference

You will learn which processor steps can be used in `processor:` pipelines and legacy `processor_raw` fallback names.

## How processors run

Processors run in the order listed. Each step receives the previous step's output and returns text that the feed parser reads as IPs, CIDRs, ranges, or source-specific intermediate data.

```yaml
processor:
  - remove_comments
  - extract_ipv4_cidr
```

Steps can also use arguments. In the YAML file, write the argument map under the processor name; this is the processor step's `args` map:

```yaml
processor:
  - grep:
      pattern: "^allow"
  - csv_column:
      index: "2"
```

The daemon automatically uses streaming implementations for processors that support streaming.

## General text processors

| Processor | Purpose | Arguments |
|---|---|---|
| `passthrough` | Leave input unchanged. | none |
| `cat`, `$CAT_CMD` | Compatibility aliases for `passthrough`. | none |
| `trim` | Trim whitespace and remove empty lines. | none |
| `remove_comments` | Remove `#` comments and blank lines. | none |
| `remove_comments_semi`, `remove_comments_semi_colon` | Remove `;` comments and blank lines. | none |
| `grep` | Keep lines matching a pattern. | `pattern` or `value`; optional `literal`, `case_insensitive` |
| `grep_not` | Drop lines matching a pattern. | `pattern` or `value`; optional `literal`, `case_insensitive` |
| `cut_delimiter` | Split each line and keep one field. | `delimiter` or `value`; optional `field` (1-based, default `1`) |
| `csv_column`, `csv_comma_first_column` | Parse CSV and keep one column. | `index` (1-based, default `1`) |

## IP extraction and filtering

| Processor | Purpose | Arguments |
|---|---|---|
| `extract_ipv4`, `extract_ipv4_from_any_file` | Extract IPv4 addresses and drop CIDR suffixes. | none |
| `extract_ipv4_cidr`, `extract_cidr`, `extract_ipv4_cidr_from_any_file` | Extract IPv4 addresses and preserve CIDR suffixes. | none |
| `subnet_to_cidr`, `subnet_to_bitmask` | Convert dotted subnet masks to CIDR prefix lengths. | none |
| `append_slash32`, `remove_slash32` | Add or remove `/32` on IPv4 host entries. | none |
| `append_slash128`, `remove_slash128` | Add or remove `/128` on IPv6 host entries. | none |
| `filter_ip4` | Keep only IPv4 host addresses. | none |
| `filter_net4` | Keep only IPv4 networks. | none |
| `filter_all4` | Keep IPv4 hosts and networks. | none |
| `filter_invalid4` | Drop invalid IPv4 entries. | none |
| `filter_ip6` | Keep only IPv6 host addresses. | none |
| `filter_net6` | Keep only IPv6 networks. | none |
| `filter_all6` | Keep IPv6 hosts and networks. | none |
| `hostname_resolve`, `hostname_resolver` | Resolve hostnames to IPv4 addresses. | `threads` (default `10`, maximum `100`) |

## Archives and structured data

| Processor | Purpose | Arguments |
|---|---|---|
| `gunzip` | Decompress gzip input. | none |
| `unzip`, `unzip_and_extract` | Extract one file from a zip archive. | optional `file`; when omitted, the first file is used |
| `unzip_csv`, `unzip_and_split_csv` | Extract the first zip member and split CSV fields into lines. | none |
| `json_path` | Extract values from one JSON path. | `path` or `value` |
| `json_paths` | Extract values from multiple JSON paths. | `paths`, `path`, or `value` |
| `regex` | Extract regex matches. If the pattern has a capture group, group 1 is returned. | `pattern` or `value` |
| `xml_tag`, `parse_xml_clean_mx` | Extract text from XML tags. | `tag` (default `ip`) |
| `xml_rss_title`, `parse_php_rss` | Extract the first IPv4 address from RSS title text. | none |
| `xml_rss_title_resolve`, `parse_rss_rosinstrument` | Extract hostnames from RSS title text and resolve them. | `threads` for resolver behavior |
| `xml_rss_proxy`, `parse_rss_proxy` | Extract proxy IPs from RSS proxy tags. | none |
| `dshield_api_xml`, `parse_dshield_api` | Extract and normalize IPs from the DShield API XML format. | none |

## Feed-specific compatibility processors

These processors preserve compatibility with feed formats inherited from the original catalog.

| Processor | Purpose |
|---|---|
| `dshield_format`, `dshield_parser` | Parse DShield `block.txt` format. |
| `snort_rules`, `snort_alert_rules_to_ipv4` | Extract IPs and networks from Snort alert rules. |
| `pix_deny_rules`, `pix_deny_rules_to_ipv4` | Extract IPs and networks from PIX deny rules. |
| `torproject_exits` | Parse Tor `exit-addresses` format. |
| `dataplane_column3` | Extract the third column from DataPlane-style feeds. |
| `p2p_blocklist`, `p2p_gz`, `p2p_blocklist_ips`, `p2p_gz_ips` | Parse gzip-compressed P2P blocklist ranges. |
| `p2p_blocklist_proxy`, `p2p_gz_proxy` | Parse only proxy ranges from gzip-compressed P2P blocklists. |
| `parse_cleantalk` | Extract CleanTalk IPs from HTML-like content. |
| `parse_cta_cryptowall` | Extract CTA CryptoWall indicators. |
| `parse_graphiclineweb` | Extract GraphiclineWeb indicators. |
| `botscout_filter` | Extract BotScout IPs from IP-check links. |
| `gz_proxyrss` | Decompress gzip RSS proxy data and parse proxy IPs. |
| `ip2location_ip2proxy_px1lite` | Extract IP2Location IP2Proxy PX1 Lite data. |
| `blueliv_parser` | Parse BlueLiv crime-server JSON. |
| `parse_cvs_clean_mx_phishing` | Parse Clean MX phishing CSV. |
| `hphosts2ips` | Extract hostnames from hpHosts-style files and resolve them. |
| `parse_client9_ipcat_datacenters` | Parse client9 ipcat datacenter CSV ranges. |
| `parse_ipblacklistcloud` | Extract IPs from IPBlacklistCloud HTML-like content. |
| `parse_maxmind_proxy_fraud` | Extract MaxMind proxy/fraud sample IPs. |
| `parse_uscert_csv` | Parse US-CERT CSV feeds. |

## Choosing a processor

Use the smallest processor chain that turns upstream data into one entry per line. Prefer the generic processors first (`remove_comments`, `extract_ipv4_cidr`, `csv_column`, `json_path`, `regex`). Use feed-specific compatibility processors only when the upstream format requires them.

For CIDR feeds, prefer `extract_ipv4_cidr` over `extract_ipv4`; `extract_ipv4` strips the prefix and turns a network into a single host address.
