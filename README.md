# Concurrent Collision Analyzer

A Go service that processes CERN Open Data (CMS dimuon events) in parallel
and computes the invariant-mass histogram of muon pairs.

## Status

Work in progress. Currently building the sequential core
(physics, histogram, CSV ingestion) before adding the concurrent pipeline.
