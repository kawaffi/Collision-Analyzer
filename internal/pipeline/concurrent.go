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

// Options configures Concurrent. Zero fields select a default (see
// withDefaults); negative values are an error.
type Options struct {
	Workers   int // goroutines that parse blocks and fill histograms
	BatchSize int // lines of input handed to a worker at a time
}

// DefaultBatchSize is the batch size used when Options.BatchSize is zero.
const DefaultBatchSize = 5000

// withDefaults returns o with zero fields replaced by their defaults: all
// available threads for Workers and DefaultBatchSize for BatchSize. Negative
// values are an error.
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

// workerResult is what a worker hands back when it has finished.
type workerResult struct {
	hist  *histogram.Histogram
	stats Stats
}

// Concurrent does the same work as Sequential using a pool of workers. The
// calling goroutine only finds line ends and groups lines in blocks; the
// workers split the lines into fields, parse them, compute the invariant mass
// and fill a private histogram; the private histograms are merged into h at
// the end. The input must have one record per line (no quoted field may
// contain a newline), as the CMS dimuon files do.
//
// Malformed rows are skipped and counted in Stats.Skipped, as in Sequential.
// Stats always describe the work done, also when an error is returned. If an
// error is returned, h is left unchanged.
//
// ctx is checked once per block: when it is cancelled, reading stops, the
// blocks already queued are processed, and ctx.Err() is returned. A Read
// blocked inside r is not interrupted.
func Concurrent(ctx context.Context, r io.Reader, h *histogram.Histogram, opts Options) (Stats, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return Stats{}, err
	}

	br, err := ingest.NewBlockReader(r, opts.BatchSize)
	if err != nil {
		return Stats{}, err
	}
	parser := br.Parser()

	blocks := make(chan ingest.Block, 2*opts.Workers)
	results := make([]workerResult, opts.Workers)

	var wg sync.WaitGroup
	for i := range results {
		local := h.NewLike()

		wg.Add(1)
		go func() {
			defer wg.Done()

			var stats Stats // kept in a local variable: no shared writes while running
			for block := range blocks {
				// Stats report how many rows were skipped; the error that
				// ParseBlock also returns (the first bad row, with its line
				// number) is for callers that want to show where.
				skipped, _ := parser.ParseBlock(block, func(ev ingest.Event) {
					local.Add(physics.InvariantMass(ev.Mu1, ev.Mu2))
					stats.Processed++
				})
				stats.Skipped += uint64(skipped)
			}
			results[i] = workerResult{hist: local, stats: stats}
		}()
	}

	// Reader: runs in the calling goroutine. Sending on a full channel blocks
	// it, so it can never run far ahead of the workers (backpressure). The
	// workers keep consuming until the channel is closed, so a send can never
	// block for good, whatever the reason the reader stops.
	var readErr error
	cancelled := ctx.Err() != nil
	for !cancelled {
		block, err := br.ReadBlock()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			readErr = err
			break
		}
		blocks <- block
		cancelled = ctx.Err() != nil
	}
	close(blocks)
	wg.Wait()

	var total Stats
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
			return total, err // cannot happen: the histograms come from NewLike
		}
	}
	return total, nil
}
