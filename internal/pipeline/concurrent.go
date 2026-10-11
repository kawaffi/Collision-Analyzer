package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
	"github.com/kawaffi/collision-analyzer/internal/ingest"
	"github.com/kawaffi/collision-analyzer/internal/physics"
)

type Options struct {
	Workers   int
	BatchSize int
}

type workerResult struct {
	hist  *histogram.Histogram
	stats Stats
}

func Concurrent(ctx context.Context, r io.Reader, h *histogram.Histogram, opts Options) (Stats, error) {
	if opts.Workers < 1 || opts.BatchSize < 1 {
		return Stats{}, fmt.Errorf("pipeline: Workers and BatchSize must be positive, got %d and %d",
			opts.Workers, opts.BatchSize)
	}

	rd, err := ingest.NewReader(r)
	if err != nil {
		return Stats{}, err
	}
	parser := rd.Parser()

	batches := make(chan []ingest.Record, 2*opts.Workers)
	results := make([]workerResult, opts.Workers)

	var wg sync.WaitGroup
	for i := range results {
		local := h.NewLike()

		wg.Add(1)
		go func() {
			defer wg.Done()

			var stats Stats
			for batch := range batches {
				for _, rec := range batch {
					ev, err := parser.Parse(rec)
					if err != nil {
						stats.Skipped++
						continue
					}
					local.Add(physics.InvariantMass(ev.Mu1, ev.Mu2))
					stats.Processed++
				}
			}
			results[i] = workerResult{hist: local, stats: stats}
		}()
	}

	var readErr error
	batch := make([]ingest.Record, 0, opts.BatchSize)
	for {
		rec, err := rd.ReadRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			readErr = err
			break
		}

		batch = append(batch, rec)
		if len(batch) == opts.BatchSize {
			batches <- batch
			batch = make([]ingest.Record, 0, opts.BatchSize)
		}
	}
	if readErr == nil && len(batch) > 0 {
		batches <- batch
	}
	close(batches)
	wg.Wait()

	var total Stats
	for _, res := range results {
		total.Processed += res.stats.Processed
		total.Skipped += res.stats.Skipped
	}
	if readErr != nil {
		return total, readErr
	}

	for _, res := range results {
		if err := h.Merge(res.hist); err != nil {
			return total, err
		}
	}
	return total, nil
}
