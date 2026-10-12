package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
	"github.com/kawaffi/collision-analyzer/internal/pipeline"
)

const barWidth = 50

type config struct {
	lo, hi     float64
	bins       int
	workers    int
	batch      int
	sequential bool
}

func main() {
	var cfg config
	flag.Float64Var(&cfg.lo, "lo", 2, "lower edge of the mass range (GeV)")
	flag.Float64Var(&cfg.hi, "hi", 110, "upper edge of the mass range (GeV)")
	flag.IntVar(&cfg.bins, "bins", 108, "number of bins")
	flag.IntVar(&cfg.workers, "workers", 0, "number of workers (0 = all available threads)")
	flag.IntVar(&cfg.batch, "batch", 0, fmt.Sprintf("rows per batch (0 = %d)", pipeline.DefaultBatchSize))
	flag.BoolVar(&cfg.sequential, "sequential", false, "use the single-threaded pipeline (ignores -workers and -batch)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: analyze [flags] file.csv [file.csv ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	err := run(ctx, flag.Args(), cfg)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, files []string, cfg config) error {
	h, err := histogram.New(cfg.lo, cfg.hi, cfg.bins)
	if err != nil {
		return err
	}

	if cfg.workers == 0 {
		cfg.workers = runtime.GOMAXPROCS(0)
	}
	if cfg.batch == 0 {
		cfg.batch = pipeline.DefaultBatchSize
	}
	mode := fmt.Sprintf("concurrent, %d workers, batches of %d rows", cfg.workers, cfg.batch)
	if cfg.sequential {
		mode = "sequential"
	}

	var total pipeline.Stats
	start := time.Now()
	for _, name := range files {
		stats, err := processFile(ctx, name, h, cfg)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		total.Processed += stats.Processed
		total.Skipped += stats.Skipped
	}
	elapsed := time.Since(start)

	fmt.Printf("mode:        %s\n", mode)
	fmt.Printf("files:       %d\n", len(files))
	fmt.Printf("processed:   %d events\n", total.Processed)
	fmt.Printf("skipped:     %d malformed rows\n", total.Skipped)
	fmt.Printf("underflow:   %d (mass below %g)\n", h.Underflow(), cfg.lo)
	fmt.Printf("overflow:    %d (mass at or above %g)\n", h.Overflow(), cfg.hi)
	fmt.Printf("invalid:     %d (NaN)\n", h.Invalid())
	fmt.Printf("elapsed:     %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput:  %.0f events/s\n\n", float64(total.Processed)/elapsed.Seconds())

	printHistogram(h)
	return nil
}

func processFile(ctx context.Context, name string, h *histogram.Histogram, cfg config) (pipeline.Stats, error) {
	f, err := os.Open(name)
	if err != nil {
		return pipeline.Stats{}, err
	}
	defer f.Close()

	if cfg.sequential {
		return pipeline.Sequential(f, h)
	}
	return pipeline.Concurrent(ctx, f, h, pipeline.Options{Workers: cfg.workers, BatchSize: cfg.batch})
}

// printHistogram draws one bar per bin, scaled logarithmically.
func printHistogram(h *histogram.Histogram) {
	counts := h.Counts()
	edges := h.BinEdges()

	var max uint64
	for _, c := range counts {
		if c > max {
			max = c
		}
	}

	fmt.Println("mass [GeV]          count  (bars: log scale)")
	for i, c := range counts {
		n := 0
		if c > 0 {
			n = 1 + int(float64(barWidth-1)*math.Log1p(float64(c))/math.Log1p(float64(max)))
		}
		fmt.Printf("%7.2f-%-7.2f %7d  %s\n", edges[i], edges[i+1], c, strings.Repeat("#", n))
	}
}
