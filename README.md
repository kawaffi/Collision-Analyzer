# Concurrent Collision Analyzer

A Go tool that computes the invariant-mass histogram of muon pairs from CERN
Open Data (CMS dimuon events). It has a sequential pipeline and a concurrent
one (batches, worker pool, per-worker histograms) which are tested against each
other and benchmarked.

## Status

- [x] Sequential core: `physics`, `histogram`, `ingest`, `pipeline.Sequential`
- [x] Concurrent pipeline with context cancellation, tested against the
      sequential one on random input under the race detector
- [x] Benchmarks (see Results)
- [x] Remove the single-reader and allocations bottlenecks found by the benchmarks
- [ ] HTTP API, web UI, Docker

## Data

Dimuon events derived from the CMS Run2010B Mu dataset, available on the
[CERN Open Data portal](https://opendata.cern.ch/record/700)
(DOI 10.7483/OPENDATA.CMS.CB8H.MFFA). Download the CSV files into `data/`.
A 20-event sample is in `testdata/`.

## Usage

    go run ./cmd/analyze data/*.csv                              # full spectrum
    go run ./cmd/analyze -lo 2 -hi 4 -bins 40 data/*.csv         # J/ψ and ψ(2S)
    go run ./cmd/analyze -lo 8 -hi 12 -bins 40 data/*.csv        # Υ family
    go run ./cmd/analyze -lo 80 -hi 100 -bins 40 data/*.csv      # Z boson
    go run ./cmd/analyze -sequential data/*.csv                  # single-threaded
    go run ./cmd/analyze -workers 4 -batch 1000 data/*.csv       # tune the pool
    go run ./cmd/analyze -h                                      # all flags

    go test -race ./...
    go test -run '^$' -bench BenchmarkPipeline -count 10 ./internal/pipeline

The computed spectrum shows the J/ψ near 3.1 GeV, the ψ(2S) near 3.7 GeV, the
Υ family around 9.5-10.4 GeV and the Z boson near 91 GeV. Ctrl-C cancels a run.

## Results

Benchmark on one million rows (the 20 sample rows repeated, generated on the
fly), 10 runs each, Intel Core i5-9600K (6 cores, 6 threads), Linux (WSL2),
Go 1.26.0.

| Pipeline             | Time   | Throughput     | Speedup vs sequential | Scaling vs 1 worker |
|----------------------|--------|----------------|-----------------------|---------------------|
| sequential           | 947 ms | 1.06M events/s | 1.00x                 |                     |
| concurrent, 1 worker | 845 ms | 1.18M events/s | 1.12x                 | 1.00x               |
| concurrent, 2        | 437 ms | 2.29M events/s | 2.17x                 | 1.93x               |
| concurrent, 4        | 227 ms | 4.40M events/s | 4.16x                 | 3.71x               |
| concurrent, 6        | 178 ms | 5.62M events/s | 5.32x                 | 4.74x               |
| concurrent, 8        | 175 ms | 5.70M events/s | 5.40x                 | 4.81x               |

The speedup over the sequential pipeline combines two effects: the concurrent
pipeline does less work per row (one allocation instead of two, which is why a
single worker is already 1.12x faster), and parallelism. The last column
isolates parallelism: 93% efficiency at 4 workers, 79% at 6. Going from 6 to 8
workers gains 1.5%, as expected on a 6-thread CPU. The cause of the remaining
inefficiency has not been measured.

### How the pipeline was improved

1. **First version.** One goroutine read rows with `encoding/csv` and sent
   records to the workers. Throughput was flat at about 1.5M events/s from 1 to
   8 workers. A CPU profile showed that goroutine spending 98% of its time in
   `encoding/csv`, while the process kept only 1.9 of 6 cores busy.
2. **Field splitting moved to the workers.** The reader now only finds line
   ends (`ingest.BlockReader`) and each worker splits its block into fields:
   4.63M events/s at 8 workers.
3. **Allocations.** A memory profile showed 3 allocations per row: two inside
   `encoding/csv` and one of my own, caused by `errors.As` moving its target to
   the heap on every row. Fixing it and reusing the slice of fields (safe,
   because each block is parsed entirely inside one worker) cut allocations by
   67% and memory by 55%, and raised throughput by another 23%: 5.70M events/s
   at 8 workers, 3.8x the first concurrent version.

Other numbers: the concurrent pipeline allocates 1.0M objects and 276 MiB per
million rows, against 2.0M and 464 MiB for the sequential one. Run-to-run
variation between sessions is about 1-2%. The benchmark input repeats the same
20 rows, so it measures processing throughput, not physics.

## Design notes

- `Histogram` is not safe for concurrent use on purpose: each worker owns one
  (created with `NewLike`) and the results are combined with `Merge`, which
  compares ranges exactly.
- If `Concurrent` returns an error, the caller's histogram is unchanged: there
  is no partial result that could pass for a complete one. `Stats` always
  describe the work done.
- Malformed rows are skipped and counted in `Stats.Skipped`; a real read error
  aborts the run.
- The context is checked once per batch. A `Read` blocked inside the underlying
  reader is not interrupted.
- Test oracle: the dataset stores the invariant mass of each event, which the
  tests compare against the value computed from the muon four-vectors.
  Recomputed masses agree with it to about 1e-4 GeV for most events; larger
  differences appear for high-energy events because the CSV stores 6
  significant digits.
- The M² clamp in `physics` never triggers on this dataset (checked on all
  100,000 events); it is only a safety net.
- `Processed` counts events added to the histogram (including under/overflow);
  `Skipped` counts rows that could not be parsed.
  - The reader only finds line ends, so the input must have one record per line
  (no quoted field may contain a newline), as the CMS dimuon files do.
  `BatchSize` counts lines of input.
  - Each block is parsed inside a single worker with its own `csv.Reader` and
  `ReuseRecord` enabled, so `Parse` must not keep the fields of a record.