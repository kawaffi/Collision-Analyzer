package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
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
