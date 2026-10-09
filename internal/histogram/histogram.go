package histogram

import (
	"fmt"
	"math"
	"slices"
)

type Histogram struct {
	lo, hi float64
	counts []uint64
}

func New(lo, hi float64, numBins int) (*Histogram, error) {
	if numBins <= 0 {
		return nil, fmt.Errorf("histogram: numBins must be positive, got %d", numBins)
	}

	if !isFinite(lo) || !isFinite(hi) {
		return nil, fmt.Errorf("histogram: range must be finite, got [%v, %v)", lo, hi)
	}
	if lo >= hi {
		return nil, fmt.Errorf("histogram: lo must be less than hi, got [%v, %v)", lo, hi)
	}

	return &Histogram{lo: lo, hi: hi, counts: make([]uint64, numBins)}, nil
}

func (h *Histogram) Counts() []uint64 {
	return slices.Clone(h.counts)
}

func (h *Histogram) Add(v float64) {
	n := float64(len(h.counts))
	i := int((v - h.lo) * n / (h.hi - h.lo))
	h.counts[i]++
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
