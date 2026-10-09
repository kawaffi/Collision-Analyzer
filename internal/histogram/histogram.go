package histogram

import (
	"fmt"
	"math"
	"slices"
)

type Histogram struct {
	lo, hi    float64
	counts    []uint64
	underflow uint64
	overflow  uint64
	invalid   uint64
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
	if math.IsNaN(v) {
		h.invalid++
		return
	}
	if v < h.lo {
		h.underflow++
		return
	}
	if v >= h.hi {
		h.overflow++
		return
	}
	n := float64(len(h.counts))
	i := int((v - h.lo) * n / (h.hi - h.lo))

	if i >= len(h.counts) {
		i = len(h.counts) - 1
	}
	h.counts[i]++
}

func (h *Histogram) Merge(other *Histogram) error {
	if other == nil {
		return fmt.Errorf("histogram: cannot merge with nil histogram")
	}
	if len(h.counts) != len(other.counts) || h.lo != other.lo || h.hi != other.hi {
		return fmt.Errorf("histogram: cannot merge different configurations: [%v, %v) with %d bins and [%v, %v) with %d bins",
			h.lo, h.hi, len(h.counts), other.lo, other.hi, len(other.counts))
	}

	for i, c := range other.counts {
		h.counts[i] += c
	}
	h.underflow += other.underflow
	h.overflow += other.overflow
	h.invalid += other.invalid
	return nil
}

func (h *Histogram) BinEdges() []float64 {
	n := len(h.counts)
	edges := make([]float64, n+1)
	for i := range edges {
		edges[i] = h.lo + float64(i)*(h.hi-h.lo)/float64(n)
	}
	edges[n] = h.hi
	return edges
}

func (h *Histogram) Invalid() uint64 { return h.invalid }

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}

func (h *Histogram) Underflow() uint64 { return h.underflow }

func (h *Histogram) Overflow() uint64 { return h.overflow }
