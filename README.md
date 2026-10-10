# Concurrent Collision Analyzer

A Go tool that processes CERN Open Data (CMS dimuon events) and computes the
invariant-mass histogram of muon pairs. The goal of the project is a concurrent
pipeline with a worker pool, benchmarked against a sequential baseline.

## Status

- [x] Sequential core: `physics`, `histogram`, `ingest`, `pipeline.Sequential`
- [ ] Concurrent pipeline (worker pool, cancellation, benchmarks)
- [ ] HTTP API, web UI, Docker

## Data

Dimuon events derived from the CMS Run2010B Mu dataset, available on the
[CERN Open Data portal](https://opendata.cern.ch/record/700)
(DOI 10.7483/OPENDATA.CMS.CB8H.MFFA). Download the CSV files into `data/`.
A 20-event sample is in `testdata/`.

## Usage

    go run ./cmd/analyze data/*.csv
    go test ./...
    go test -run '^$' -bench Sequential ./internal/pipeline

## Design notes

- `Histogram` is not safe for concurrent use on purpose: each worker will own
  one and the results are combined with `Merge`.
- Test oracle: the dataset stores the invariant mass of each event, which the
  tests compare against the value computed from the muon four-vectors.
- `Processed` counts events added to the histogram (including under/overflow);
  `Skipped` counts rows that could not be parsed.
- Sequential baseline: about 1.5M events/s (range 1.42-1.64M over 5 runs,
  500,000 rows, Intel Core i7-1255U, `go test -bench Sequential`).
