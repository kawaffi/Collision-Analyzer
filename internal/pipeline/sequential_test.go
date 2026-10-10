package pipeline

import (
	"slices"
	"strings"
	"testing"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
)

const header = "Run,Event,Type1,E1,px1 ,py1,pz1,pt1,eta1,phi1,Q1,Type2,E2,px2,py2,pz2,pt2,eta2,phi2,Q2,M\n"

const (
	row1 = "146436,90830792,G,19.1712,3.81713,9.04323,-16.4673,9.81583,-1.28942,1.17139,1,T,5.43984,-0.362592,2.62699,-4.74849,2.65189,-1.34587,1.70796,1,2.73205\n"
	row2 = "146436,90862225,G,12.9435,5.12579,-3.98369,-11.1973,6.4918,-1.31335,-0.660674,-1,G,11.8636,4.78984,-6.26222,-8.86434,7.88403,-0.966622,-0.917841,1,3.10256\n"
	row3 = "146436,90644850,G,12.3999,-0.849742,9.4011,8.04015,9.43943,0.77258,1.66094,1,G,8.55532,-4.85155,6.97696,-0.983229,8.49797,-0.115445,2.17841,-1,9.41149\n"
)

func newHistogram(t *testing.T) *histogram.Histogram {
	t.Helper()

	h, err := histogram.New(0, 20, 20)
	if err != nil {
		t.Fatalf("histogram.New() returned error: %v", err)
	}
	return h
}

func TestSequentialProcessesValidEvents(t *testing.T) {
	h := newHistogram(t)

	stats, err := Sequential(strings.NewReader(header+row1+row2+row3), h)
	if err != nil {
		t.Fatalf("Sequential() returned error: %v", err)
	}

	if want := (Stats{Processed: 3, Skipped: 0}); stats != want {
		t.Errorf("Sequential() stats = %+v, want %+v", stats, want)
	}

	want := make([]uint64, 20)
	want[2], want[3], want[9] = 1, 1, 1
	if got := h.Counts(); !slices.Equal(got, want) {
		t.Errorf("Counts() = %v, want %v", got, want)
	}
}
