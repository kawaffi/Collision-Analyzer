package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
	"github.com/kawaffi/collision-analyzer/internal/pipeline"
)

const barWidth = 50

func main() {
	lo := flag.Float64("lo", 2, "lower edge of the mass range (GeV)")
	hi := flag.Float64("hi", 110, "upper edge of the mass range (GeV)")
	bins := flag.Int("bins", 108, "number of bins")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: analyze [flags] file.csv [file.csv ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Args(), *lo, *hi, *bins); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(files []string, lo, hi float64, bins int) error {
	h, err := histogram.New(lo, hi, bins)
	if err != nil {
		return err
	}

	var total pipeline.Stats
	start := time.Now()
	for _, name := range files {
		stats, err := processFile(name, h)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		total.Processed += stats.Processed
		total.Skipped += stats.Skipped
	}
	elapsed := time.Since(start)

	fmt.Printf("files:       %d\n", len(files))
	fmt.Printf("processed:   %d events\n", total.Processed)
	fmt.Printf("skipped:     %d malformed rows\n", total.Skipped)
	fmt.Printf("underflow:   %d (mass below %g)\n", h.Underflow(), lo)
	fmt.Printf("overflow:    %d (mass at or above %g)\n", h.Overflow(), hi)
	fmt.Printf("invalid:     %d (NaN)\n", h.Invalid())
	fmt.Printf("elapsed:     %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput:  %.0f events/s\n\n", float64(total.Processed)/elapsed.Seconds())

	printHistogram(h)
	return nil
}

func processFile(name string, h *histogram.Histogram) (pipeline.Stats, error) {
	f, err := os.Open(name)
	if err != nil {
		return pipeline.Stats{}, err
	}
	defer f.Close()

	return pipeline.Sequential(f, h)
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
