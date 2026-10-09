package histogram

import (
	"fmt"
	"math"
)

type Histogram struct{}

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

	return &Histogram{}, nil
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
