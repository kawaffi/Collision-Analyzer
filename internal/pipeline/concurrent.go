package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
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

// DefaultBatchSize is the batch size used when Options.BatchSize is zero.
const DefaultBatchSize = 5000

func (o Options) withDefaults() (Options, error) {
	if o.Workers < 0 || o.BatchSize < 0 {
		return Options{}, fmt.Errorf("pipeline: Workers and BatchSize must not be negative, got %d and %d",
			o.Workers, o.BatchSize)
	}
	if o.Workers == 0 {
		o.Workers = runtime.GOMAXPROCS(0)
	}
	if o.BatchSize == 0 {
		o.BatchSize = DefaultBatchSize
	}
	return o, nil
}

func Concurrent(ctx context.Context, r io.Reader, h *histogram.Histogram, opts Options) (Stats, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return Stats{}, err
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

	cancelled := ctx.Err() != nil

	var readerSkipped uint64
	var readErr error
	batch := make([]ingest.Record, 0, opts.BatchSize)
	for !cancelled {
		rec, err := rd.ReadRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, ingest.ErrMalformedRow) {
			readerSkipped++
			continue
		}
		if err != nil {
			readErr = err
			break
		}

		batch = append(batch, rec)
		if len(batch) == opts.BatchSize {
			batches <- batch
			batch = make([]ingest.Record, 0, opts.BatchSize)
			cancelled = ctx.Err() != nil
		}
	}
	if readErr == nil && !cancelled && len(batch) > 0 {
		batches <- batch
	}
	close(batches)
	wg.Wait()

	total := Stats{Skipped: readerSkipped}
	for _, res := range results {
		total.Processed += res.stats.Processed
		total.Skipped += res.stats.Skipped
	}
	if readErr != nil {
		return total, readErr
	}
	if err := ctx.Err(); err != nil {
		return total, err
	}

	for _, res := range results {
		if err := h.Merge(res.hist); err != nil {
			return total, err
		}
	}
	return total, nil
}
