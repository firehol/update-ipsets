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

The current benchmark payload mirrors the legacy comparison-pair ledger JSON
shape at production scale: 400 feeds produce 79,800 pair entries.
