# JSON Benchmark Tool

This module benchmarks third-party JSON codecs against project-shaped payloads
without adding those codecs to the main application dependency graph.

Run:

```bash
make jsonbench
```

For production-grade comparisons, run repeated samples and compare with
`benchstat`, for example:

```bash
cd tools/jsonbench
go test -run '^$' -bench=. -benchmem -count=10 ./... > /tmp/update-ipsets-jsonbench.txt
```

The benchmark suite includes:

- legacy comparison-pair ledger JSON at production scale: 400 feeds produce
  79,800 pair entries;
- project-shaped feed entity sidecar, ASN detail, cache state, and scheduler
  snapshot payloads.

The compatibility tests include a child-process crash reproducer for
`velox-io/json` v0.1.4 cache-state-shaped marshal. Velox cache-state benchmark
rows are skipped until that crash is resolved.
