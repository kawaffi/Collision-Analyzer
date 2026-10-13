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
- [x] Remove the single-reader bottleneck found by the benchmarks
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
Go <go version>. Time per run and speedup over the sequential pipeline:

| Pipeline             | Time   | Throughput     | Speedup |
|----------------------|--------|----------------|---------|
| sequential           | 934 ms | 1.07M events/s | 1.00x   |
| concurrent, 1 worker | 997 ms | 1.00M events/s | 0.94x   |
| concurrent, 2        | 531 ms | 1.88M events/s | 1.76x   |
| concurrent, 4        | 285 ms | 3.51M events/s | 3.28x   |
| concurrent, 6        | 221 ms | 4.53M events/s | 4.23x   |
| concurrent, 8        | 216 ms | 4.63M events/s | 4.33x   |

### How the first version was improved

The first concurrent version read rows with `encoding/csv` in a single
goroutine and sent the records to the workers. Its throughput was flat at about
1.5M events/s (1.41x) from 1 to 8 workers. A CPU profile showed why: that
goroutine spent 98% of its time inside `encoding/csv` and was busy for the
whole run, while the process kept only about 1.9 of 6 cores busy on average.

The fix was to make the reader only find line ends (`ingest.BlockReader`, which
is cheap) and to let each worker split its block into fields. Throughput at 8
workers went from 1.50M to 4.63M events/s, 3.1x higher than the first
concurrent version.

### What the numbers also show

- With one worker the pipeline is about 7% slower than the sequential one:
  there is nothing left to overlap, and the block copy and the per-block
  `encoding/csv` reader are pure overhead.
- Going from 6 to 8 workers gains 2%, as expected on a 6-thread CPU. At 6
  workers the efficiency is about 70%; the cause has not been measured yet
  (garbage collection and lower clock under full load are candidates).
- Memory: +33% bytes and +50% allocations per run compared to the sequential
  version (the lines are copied into blocks).
- The benchmark input repeats the same 20 rows, so it measures processing
  throughput, not physics.

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