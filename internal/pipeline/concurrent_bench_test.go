package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
)

const benchRows = 1_000_000

func benchInput(b *testing.B) func() io.Reader {
	b.Helper()

	sample := readSample(b)
	head, body, _ := bytes.Cut(sample, []byte("\n"))
	perCopy := bytes.Count(body, []byte("\n"))
	if benchRows%perCopy != 0 {
		b.Fatalf("the sample has %d rows, which does not divide %d", perCopy, benchRows)
	}
	header, rows := string(head)+"\n", string(body)
	copies := benchRows / perCopy

	return func() io.Reader {
		return io.MultiReader(
			strings.NewReader(header),
			io.LimitReader(&repeatReader{s: rows}, int64(copies*len(rows))),
		)
	}
}

func runBench(b *testing.B, input func() io.Reader, process func(io.Reader, *histogram.Histogram) (Stats, error)) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h, err := histogram.New(2, 110, 108)
		if err != nil {
			b.Fatal(err)
		}
		stats, err := process(input(), h)
		if err != nil {
			b.Fatal(err)
		}
		if stats.Processed != benchRows || stats.Skipped != 0 {
			b.Fatalf("stats = %+v, want %d processed and none skipped", stats, benchRows)
		}
	}
	b.ReportMetric(float64(benchRows)*float64(b.N)/b.Elapsed().Seconds(), "events/s")
}
func BenchmarkPipeline(b *testing.B) {
	input := benchInput(b)

	b.Run("sequential", func(b *testing.B) {
		runBench(b, input, Sequential)
	})
	for _, workers := range []int{1, 2, 4, 6, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			runBench(b, input, func(r io.Reader, h *histogram.Histogram) (Stats, error) {
				return Concurrent(context.Background(), r, h, Options{Workers: workers, BatchSize: DefaultBatchSize})
			})
		})
	}
}
func BenchmarkBatchSize(b *testing.B) {
	input := benchInput(b)
	workers := runtime.GOMAXPROCS(0)

	for _, batchSize := range []int{10, 100, 1000, 5000, 50000} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			runBench(b, input, func(r io.Reader, h *histogram.Histogram) (Stats, error) {
				return Concurrent(context.Background(), r, h, Options{Workers: workers, BatchSize: batchSize})
			})
		})
	}
}
