# Bounded live report monitoring at 10k, 100k, and 1m rows

## Hypothesis

Streaming CSV validation and bounded chart/table projections should keep retained
report data, HTML payload, and rendered elements nearly constant as logs grow from
10,000 to 100,000 to 1,000,000 rows. An unchanged live refresh should reuse its
projection. A changed file still validates and samples its entire captured prefix,
so cold parsing and append refresh time should grow with input size. This experiment
measures those costs separately; it does not assume that append refresh is constant
time.

## Method

The starting revision is `8a4bdf2c3d09793a12fdada6f8b824c237a3487a` plus the working
changes implementing live monitoring. Before each run, `run.sh` records the revision,
working-tree diff summary, Go version, OS/architecture, and benchmark command in
`artifacts/environment.txt`.
The runner hashes all internal/CLI sources and module files before and after the
batch and fails if that source fingerprint changes while measuring.

The initial environment is Go 1.27.1 on macOS Darwin 25.6.0, arm64. Fixtures are
generated in temporary directories before timing or heap observation, without
building a complete CSV byte slice/string. Each fixture contains one CSV with a
strictly increasing step column, two loss series, one throughput series, isolated
positive/negative extrema, and five missing-value runs. The report has two charts
sharing the same source, selecting two loss series and one throughput series.
The three fixture sizes are exactly 10,000, 100,000, and 1,000,000 recorded rows.
Every row is newline terminated. Appended rows are written outside the measured
interval; the refresh itself is measured. “Cold” means a fresh application reader
and handler; the operating system's filesystem cache is not flushed.

`internal/report/monitor_benchmark_test.go` measures strict static `Read`, cached
unchanged live `Reader.ReadMany`, and live refresh after one row is appended.
`internal/cli/monitor_benchmark_test.go` measures the full live HTTP pipeline
(snapshot load, conditional response, and HTML rendering) for cold, unchanged,
and appended inputs, and rendering an already loaded snapshot. Unchanged requests
send `If-None-Match`; their response bytes are recorded independently from the
complete built HTML size. The fixed timing command uses `-run '^$' -bench '^BenchmarkMonitor'
-benchtime 3x -benchmem`, with three independent process runs. It reports ns/op,
B/op (cumulative allocated bytes per operation), allocs/op, input CSV bytes,
HTML bytes, sampled plot rows, and recent raw table rows. Timing is descriptive
evidence, never a regression assertion or release gate.

A separate opt-in `TestMonitorMemory` observes cold, unchanged, and append full live
refreshes after fixture generation and a garbage collection. It samples Go
`runtime.MemStats.HeapAlloc` every millisecond, including the start/end checkpoints,
and reports the highest observed heap increase and the additional heap retained
after a forced garbage collection while the reader and result stay reachable.
These values describe Go heap usage; neither B/op nor sampled heap is resident
memory (RSS), and the sampling interval may miss a short peak. This instrumentation
runs separately from timing benchmarks and ordinary deterministic correctness
tests.

Set `EXPLEDGER_MONITOR_ARTIFACTS` to an output directory to save the bounded HTML
fixtures for actual browser DOM measurements. Set `EXPLEDGER_MONITOR_FIXTURE_DIR`
to retain a streaming copy of the 1m-row project for browser live-refresh checks.
Browser element counts must be read
after JavaScript has rendered the charts, both before and after expanding the raw
tables; counting HTML tags alone cannot observe dynamically created SVG elements.
The experiment records those measurements separately when available.

Run from the prepared checkout after deterministic loader, CLI, and browser checks:

```sh
expledger run 20261006-live-report-monitoring
```

`run.sh` owns all workload parameters and writes small text results to `artifacts/`.
The Go build cache, bounded HTML, and generated large CSV logs default to temporary
directories. The optional HTML/project output paths above are used for browser
checks, outside Git. The runner adds no dependencies and leaves input logs out of
Git.

## Finding

The final recorded batch completed successfully on 2026-10-07 UTC after the
deterministic/race/browser checks and independent review. It replaces an initial
provisional batch during which a browser-script cleanup changed the source.
The final source fingerprints before and after all timing/heap measurements agree:
`c7cd38d9ee4c751815a0110140daff48881616347d321ed873bdaad2162c8ed7`.
Results below are
medians of the three independent timing-process runs (three operations per case
in each run). The Apple M5 Max host, exact commands, and working diff summary are
recorded in [environment.txt](artifacts/environment.txt). Raw results are in
[timing-1.txt](artifacts/timing-1.txt), [timing-2.txt](artifacts/timing-2.txt),
[timing-3.txt](artifacts/timing-3.txt), and [heap.txt](artifacts/heap.txt).

| Recorded rows | Static parse (ms) | Cached reader (ms) | Append reader (ms) | Cold HTTP (ms) | Unchanged HTTP (ms) | Append HTTP (ms) |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10,000 | 4.803 | 0.167 | 3.926 | 5.506 | 0.447 | 4.816 |
| 100,000 | 29.242 | 0.161 | 29.196 | 30.388 | 0.441 | 30.418 |
| 1,000,000 | 284.248 | 0.169 | 282.901 | 290.549 | 0.438 | 291.713 |

All unchanged HTTP cases returned `304` with **zero response body bytes**. The
cached reader's median allocation count was 311 at every input size; median
unchanged HTTP allocation counts were 573, 562, and 562 respectively. Preloaded
HTML rendering took 0.270, 0.274, and 0.273 ms respectively.

| Recorded rows | Input CSV bytes | Built HTML bytes | Loss plot rows | Throughput plot rows | Raw rows per table |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 10,000 | 258,672 | 75,377 | 458 | 453 | 100 |
| 100,000 | 2,688,672 | 76,066 | 492 | 390 | 100 |
| 1,000,000 | 27,888,672 | 74,421 | 311 | 490 | 100 |

The plot count is bounded by 512 rows per chart, and table data is exactly the
latest 100 recorded rows. HTML size stays within 74–77 KB even when the CSV grows
to 27.9 MB. The row counts above are Go projection measurements.

Actual in-app browser measurements after JavaScript chart rendering are saved in
[browser-counts.json](artifacts/browser-counts.json). The report was selected, and
counts were checked with both recent-data tables collapsed and expanded; opening
the tables left the DOM counts unchanged. Each size has exactly 200 raw `tbody`
rows and 500 raw cells across the two tables. Report/SVG counts exclude their root
elements; chart dot counts include recorded observations and exclude the three
inspection-marker circles.

| Recorded rows | Page elements | Report elements | Chart SVG descendants | Chart dots |
| ---: | ---: | ---: | ---: | ---: |
| 10,000 | 2,231 | 2,163 | 1,406 | 1,366 |
| 100,000 | 2,236 | 2,168 | 1,411 | 1,371 |
| 1,000,000 | 1,975 | 1,907 | 1,150 | 1,110 |

The browser also confirmed that partial trailing rows stay hidden until completed,
selected x/focus/table disclosure/scroll survive append and continuing atomic
replacement, invalid NaN keeps the entire previous page visible with an error, and
shrinking/restarting the source clears stale selected x.

| Recorded rows | Cold HTTP allocated B/op | Unchanged HTTP allocated B/op | Highest sampled Go heap bytes (cold/append) | Go heap bytes retained after cold request + GC |
| ---: | ---: | ---: | ---: | ---: |
| 10,000 | 6,469,984 | 113,861 | 3,791,720 | 1,077,720 |
| 100,000 | 14,647,730 | 84,346 | 3,936,512 | 1,079,552 |
| 1,000,000 | 87,551,037 | 84,346 | 3,981,104 | 1,082,680 |

The separate heap observation found approximately 4 MB at its sampled peak and
1.08 MB retained after a cold request across all sizes. An unchanged HTTP request
added approximately 180 KB at the sampled peak and 32 KB after GC. Cumulative
allocated bytes on a cold/append scan still grow with the input: the 1m-row cold
request allocates about 87.6 MB over its lifetime while garbage collection keeps
live heap bounded. These measurements support the bounded-data/HTML/heap
hypothesis and cached unchanged polling staying nearly independent of row count.
Append refresh remains a full-prefix validation/sampling pass, around
292 ms at 1m rows on this host. No claim of constant-time append refresh, RSS
measurement, or speedup over the previous implementation follows from this run.

The timing workload contains a single experiment and two charts. It does not
establish constant refresh cost as catalog/source count grows: each poll still
discovers and validates catalog metadata, and sources beyond the bounded cache
can be rescanned. The saved text/JSON artifacts total 25,698 bytes; generated
CSV/HTML browser fixtures remain outside Git.
