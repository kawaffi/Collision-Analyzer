package histogram

import (
	"math"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		lo, hi  float64
		numBins int
		wantErr bool
	}{
		{"valid configuration", 0, 100, 10, false},
		{"zero bins", 0, 100, 0, true},
		{"negative bins", 0, 100, -1, true},
		{"lo equal to hi", 5, 5, 10, true},
		{"lo greater than hi", 10, 0, 10, true},
		{"NaN lo", math.NaN(), 100, 10, true},
		{"NaN hi", 0, math.NaN(), 10, true},
		{"infinite lo", math.Inf(-1), 100, 10, true},
		{"infinite hi", 0, math.Inf(1), 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := New(tt.lo, tt.hi, tt.numBins)

			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && h != nil {
				t.Errorf("New() returned a histogram together with an error")
			}
			if !tt.wantErr && h == nil {
				t.Errorf("New() returned nil histogram without an error")
			}
		})
	}
}

func mustNew(t *testing.T, lo, hi float64, numBins int) *Histogram {
	t.Helper()

	h, err := New(lo, hi, numBins)
	if err != nil {
		t.Fatalf("New(%v, %v, %d) failed: %v", lo, hi, numBins, err)
	}
	return h
}

func TestNewHasEmptyBins(t *testing.T) {
	h := mustNew(t, 0, 100, 10)

	counts := h.Counts()

	if len(counts) != 10 {
		t.Fatalf("len(Counts()) = %d, want 10", len(counts))
	}
	for i, c := range counts {
		if c != 0 {
			t.Errorf("Counts()[%d] = %d, want 0", i, c)
		}
	}
}

func TestCountsReturnsCopy(t *testing.T) {
	h := mustNew(t, 0, 100, 10)

	h.Counts()[0] = 42

	if got := h.Counts()[0]; got != 0 {
		t.Errorf("modifying the slice returned by Counts() changed the histogram: got %d, want 0", got)
	}
}
