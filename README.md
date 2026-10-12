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
- [ ] Remove the single-reader bottleneck found by the benchmarks
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
Go 1.26.0:

| Pipeline             | Time    | Throughput     | Speedup |
|----------------------|---------|----------------|---------|
| sequential           | 947 ms  | 1.06M events/s | 1.00x   |
| concurrent, 1 worker | 672 ms  | 1.49M events/s | 1.41x   |
| concurrent, 2        | 677 ms  | 1.48M events/s | 1.40x   |
| concurrent, 4        | 680 ms  | 1.47M events/s | 1.39x   |
| concurrent, 6        | 672 ms  | 1.49M events/s | 1.41x   |
| concurrent, 8        | 672 ms  | 1.49M events/s | 1.41x   |

Throughput does not grow with the number of workers: the single goroutine
that reads the file is the bottleneck. A CPU profile of the 6-worker run
confirms it: that goroutine spends about 98% of its time inside `encoding/csv`
(splitting each row into fields and allocating them) and is busy for the whole
run, while the process keeps only about 1.9 of 6 cores busy on average. The
workers, which parse the numbers and fill the histograms, account for less
than a third of the CPU time.

With one worker, reading and processing already overlap as a two-stage
pipeline (1.41x). The timings put reading at about 70% of the sequential time,
and Amdahl's law then caps the speedup at 1/0.7 ≈ 1.4x, which is what is
measured.

The concurrent version also allocates about 7% more memory (one 32-byte record
per row held in batches). Both versions make two allocations per row, which
come from `encoding/csv`.

Next step: let the reader only find line boundaries and move the field
splitting to the workers.

The benchmark input repeats the same 20 rows, so it measures processing
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