# historyaudit

`historyaudit` builds a read-only manifest for update-ipsets feed history and
retention artifacts under a `lib/` directory.

Use it before and after any history or retention migration experiment:

```bash
go run ./tools/historyaudit -lib-dir /opt/update-ipsets/lib > history-manifest.json
```

The manifest records:

- relative artifact paths
- file sizes, mtimes, and SHA-256 checksums
- history and retention CSV row counts
- first/last ledger timestamps
- whether checked ledger timestamps are monotonic
- retention cohort file counts

The tool does not read or print raw feed contents. It does not modify, compact,
delete, rewrite, or migrate any runtime artifact.

By default the manifest omits the absolute `lib/` path so it can be committed or
shared without exposing local deployment paths. Use `-include-root` only for
local operator notes that will not be committed.
