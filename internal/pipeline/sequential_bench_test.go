package pipeline

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
)

// BenchmarkSequential measures the single-threaded baseline on 500,000 rows,
// built by repeating the rows of testdata/sample.csv. The rows repeat, so this
// measures parsing and histogramming throughput, not anything physical.
func BenchmarkSequential(b *testing.B) {
	const copies = 25_000

	sample, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.csv"))
	if err != nil {
		b.Fatalf("reading sample file: %v", err)
	}
	head, rows, _ := bytes.Cut(sample, []byte("\n"))
	perCopy := bytes.Count(rows, []byte("\n"))

	var buf bytes.Buffer
	buf.Write(head)
	buf.WriteByte('\n')
	for i := 0; i < copies; i++ {
		buf.Write(rows)
	}
	data := buf.Bytes()
	wantEvents := uint64(copies * perCopy)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h, err := histogram.New(2, 110, 108)
		if err != nil {
			b.Fatal(err)
		}
		stats, err := Sequential(bytes.NewReader(data), h)
		if err != nil {
			b.Fatal(err)
		}
		if stats.Processed != wantEvents {
			b.Fatalf("processed %d events, want %d", stats.Processed, wantEvents)
		}
	}
	b.ReportMetric(float64(wantEvents)*float64(b.N)/b.Elapsed().Seconds(), "events/s")
}
