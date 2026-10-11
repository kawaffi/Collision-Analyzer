package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
	"github.com/kawaffi/collision-analyzer/internal/ingest"
)

func readSample(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.csv"))
	if err != nil {
		t.Fatalf("reading sample file: %v", err)
	}
	return data
}

func assertSameHistogram(t *testing.T, got, want *histogram.Histogram) {
	t.Helper()

	if !slices.Equal(got.Counts(), want.Counts()) {
		t.Errorf("Counts() = %v, want %v", got.Counts(), want.Counts())
	}
	if got.Underflow() != want.Underflow() {
		t.Errorf("Underflow() = %d, want %d", got.Underflow(), want.Underflow())
	}
	if got.Overflow() != want.Overflow() {
		t.Errorf("Overflow() = %d, want %d", got.Overflow(), want.Overflow())
	}
	if got.Invalid() != want.Invalid() {
		t.Errorf("Invalid() = %d, want %d", got.Invalid(), want.Invalid())
	}
}

func TestConcurrentMatchesSequentialOnSample(t *testing.T) {
	data := readSample(t)

	want := newHistogram(t)
	wantStats, err := Sequential(bytes.NewReader(data), want)
	if err != nil {
		t.Fatalf("Sequential() returned error: %v", err)
	}

	// The sample has 20 rows: the batch sizes cover one row per batch, sizes
	// that do not divide 20, exactly 20, and larger than the whole input.
	for _, workers := range []int{1, 2, 4, 8} {
		for _, batchSize := range []int{1, 3, 7, 20, 21, 5000} {
			t.Run(fmt.Sprintf("workers=%d/batch=%d", workers, batchSize), func(t *testing.T) {
				got := newHistogram(t)

				stats, err := Concurrent(context.Background(), bytes.NewReader(data), got,
					Options{Workers: workers, BatchSize: batchSize})
				if err != nil {
					t.Fatalf("Concurrent() returned error: %v", err)
				}

				if stats != wantStats {
					t.Errorf("Concurrent() stats = %+v, want %+v", stats, wantStats)
				}
				assertSameHistogram(t, got, want)
			})
		}
	}
}

func TestConcurrentAddsToExistingHistogram(t *testing.T) {
	data := readSample(t)

	want := newHistogram(t)
	want.Add(5)
	if _, err := Sequential(bytes.NewReader(data), want); err != nil {
		t.Fatalf("Sequential() returned error: %v", err)
	}

	got := newHistogram(t)
	got.Add(5)
	if _, err := Concurrent(context.Background(), bytes.NewReader(data), got,
		Options{Workers: 4, BatchSize: 3}); err != nil {
		t.Fatalf("Concurrent() returned error: %v", err)
	}

	assertSameHistogram(t, got, want)
}

func TestConcurrentRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{"negative workers", Options{Workers: -1, BatchSize: 10}},
		{"negative batch size", Options{Workers: 2, BatchSize: -5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHistogram(t)

			_, err := Concurrent(context.Background(), strings.NewReader(header+row1), h, tt.opts)

			if err == nil {
				t.Fatal("Concurrent() returned nil error, want an error")
			}
			if !slices.Equal(h.Counts(), make([]uint64, 20)) {
				t.Errorf("histogram modified despite the error: Counts() = %v", h.Counts())
			}
		})
	}
}

func TestOptionsWithDefaults(t *testing.T) {
	cpus := runtime.GOMAXPROCS(0)

	tests := []struct {
		name    string
		in      Options
		want    Options
		wantErr bool
	}{
		{"all zero", Options{}, Options{Workers: cpus, BatchSize: DefaultBatchSize}, false},
		{"explicit values are kept", Options{Workers: 3, BatchSize: 10}, Options{Workers: 3, BatchSize: 10}, false},
		{"only workers set", Options{Workers: 3}, Options{Workers: 3, BatchSize: DefaultBatchSize}, false},
		{"only batch size set", Options{BatchSize: 10}, Options{Workers: cpus, BatchSize: 10}, false},
		{"negative workers", Options{Workers: -1}, Options{}, true},
		{"negative batch size", Options{BatchSize: -1}, Options{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.withDefaults()

			if (err != nil) != tt.wantErr {
				t.Fatalf("withDefaults() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("withDefaults() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConcurrentWithDefaultOptions(t *testing.T) {
	data := readSample(t)

	want := newHistogram(t)
	wantStats, err := Sequential(bytes.NewReader(data), want)
	if err != nil {
		t.Fatalf("Sequential() returned error: %v", err)
	}

	got := newHistogram(t)
	stats, err := Concurrent(context.Background(), bytes.NewReader(data), got, Options{})
	if err != nil {
		t.Fatalf("Concurrent() returned error: %v", err)
	}

	if stats != wantStats {
		t.Errorf("Concurrent() stats = %+v, want %+v", stats, wantStats)
	}
	assertSameHistogram(t, got, want)
}

func TestConcurrentSkipsMalformedRows(t *testing.T) {
	// Two families of bad rows. Wrong field counts are noticed by the reader
	// (ReadRecord); unreadable numbers, NaN and Inf are noticed by the workers
	// (Parse). Both must be skipped and counted, exactly as Sequential does.
	var (
		tooFew  = "146436,90830792,G\n"
		tooMany = strings.TrimSuffix(row1, "\n") + ",extra\n"
		badNum  = strings.Replace(row1, "19.1712", "abc", 1)
		nanRow  = strings.Replace(row1, "19.1712", "NaN", 1)
		infRow  = strings.Replace(row1, "-16.4673", "+Inf", 1)
		emptyM  = strings.Replace(row1, "2.73205", "", 1)
	)

	input := header +
		tooFew + row1 + badNum + row2 + nanRow + tooMany + row3 + row1 + infRow + emptyM + row2 + tooFew
	const wantProcessed, wantSkipped = 5, 7

	want := newHistogram(t)
	wantStats, err := Sequential(strings.NewReader(input), want)
	if err != nil {
		t.Fatalf("Sequential() returned error: %v", err)
	}
	if wantStats != (Stats{Processed: wantProcessed, Skipped: wantSkipped}) {
		t.Fatalf("Sequential() stats = %+v, test input is not what the test assumes", wantStats)
	}

	for _, workers := range []int{1, 2, 4, 8} {
		for _, batchSize := range []int{1, 2, 3, 100} {
			t.Run(fmt.Sprintf("workers=%d/batch=%d", workers, batchSize), func(t *testing.T) {
				got := newHistogram(t)

				stats, err := Concurrent(context.Background(), strings.NewReader(input), got,
					Options{Workers: workers, BatchSize: batchSize})
				if err != nil {
					t.Fatalf("Concurrent() returned error: %v", err)
				}

				if stats != wantStats {
					t.Errorf("Concurrent() stats = %+v, want %+v", stats, wantStats)
				}
				assertSameHistogram(t, got, want)
			})
		}
	}
}

func TestConcurrentReturnsReadErrorWithoutMergingPartialResult(t *testing.T) {
	boom := errors.New("disk on fire")
	const validRows = 200
	input := header + strings.Repeat(row1, validRows)

	for _, workers := range []int{1, 4} {
		for _, batchSize := range []int{1, 7, 5000} {
			t.Run(fmt.Sprintf("workers=%d/batch=%d", workers, batchSize), func(t *testing.T) {
				h := newHistogram(t)
				in := io.MultiReader(strings.NewReader(input), iotest.ErrReader(boom))

				stats, err := Concurrent(context.Background(), in, h,
					Options{Workers: workers, BatchSize: batchSize})

				if !errors.Is(err, boom) {
					t.Errorf("Concurrent() error = %v, want %v", err, boom)
				}
				if errors.Is(err, ingest.ErrMalformedRow) {
					t.Error("a read error was reported as a malformed row")
				}
				assertSameHistogram(t, h, newHistogram(t))

				if stats.Processed > validRows || stats.Skipped != 0 {
					t.Errorf("Concurrent() stats = %+v, want Processed <= %d and Skipped = 0", stats, validRows)
				}
			})
		}
	}
}
