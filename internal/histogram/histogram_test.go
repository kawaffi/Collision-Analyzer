package histogram

import (
	"math"
	"math/rand/v2"
	"slices"
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

func TestAddCountsValueInMatchingBin(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantBin int
	}{
		{"first bin", 5, 0},
		{"middle bin", 55, 5},
		{"last bin", 95, 9},
		{"lower edge of the range", 0, 0},
		{"value on a bin boundary goes to the upper bin", 50, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mustNew(t, 0, 100, 10)

			h.Add(tt.value)

			want := make([]uint64, 10)
			want[tt.wantBin] = 1
			if got := h.Counts(); !slices.Equal(got, want) {
				t.Errorf("Counts() = %v, want %v", got, want)
			}
		})
	}
}

func TestAddOutOfRange(t *testing.T) {
	tests := []struct {
		name          string
		value         float64
		wantUnderflow uint64
		wantOverflow  uint64
	}{
		{"below lo", -5, 1, 0},
		{"equal to hi", 100, 0, 1},
		{"above hi", 150, 0, 1},
		{"just below lo", math.Nextafter(0, -1), 1, 0},
		{"negative infinity", math.Inf(-1), 1, 0},
		{"positive infinity", math.Inf(1), 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mustNew(t, 0, 100, 10)

			h.Add(tt.value)

			if got := h.Underflow(); got != tt.wantUnderflow {
				t.Errorf("Underflow() = %d, want %d", got, tt.wantUnderflow)
			}
			if got := h.Overflow(); got != tt.wantOverflow {
				t.Errorf("Overflow() = %d, want %d", got, tt.wantOverflow)
			}
			if got := h.Counts(); !slices.Equal(got, make([]uint64, 10)) {
				t.Errorf("out-of-range value changed the bins: Counts() = %v", got)
			}
		})
	}
}

func TestAddJustBelowHiStaysInLastBin(t *testing.T) {
	for _, lo := range []float64{-1, 0, 0.1, 1, 1000} {
		for _, width := range []float64{0.3, 1, 7, 100} {
			hi := lo + width

			for numBins := 1; numBins <= 50; numBins++ {
				h := mustNew(t, lo, hi, numBins)

				h.Add(math.Nextafter(hi, math.Inf(-1)))

				if got := h.Counts()[numBins-1]; got != 1 || h.Overflow() != 0 {
					t.Errorf("lo=%v hi=%v numBins=%d: last bin = %d, overflow = %d, want 1 and 0",
						lo, hi, numBins, got, h.Overflow())
				}
			}
		}
	}
}

func TestAddNaNIsCountedSeparately(t *testing.T) {
	h := mustNew(t, 0, 100, 10)

	h.Add(math.NaN())
	h.Add(math.NaN())

	if got := h.Invalid(); got != 2 {
		t.Errorf("Invalid() = %d, want 2", got)
	}
	if h.Underflow() != 0 || h.Overflow() != 0 {
		t.Errorf("NaN counted as out of range: underflow = %d, overflow = %d", h.Underflow(), h.Overflow())
	}
	if got := h.Counts(); !slices.Equal(got, make([]uint64, 10)) {
		t.Errorf("NaN changed the bins: Counts() = %v", got)
	}
}

func TestMergeSumsBins(t *testing.T) {
	a := mustNew(t, 0, 100, 10)
	b := mustNew(t, 0, 100, 10)
	a.Add(5)
	a.Add(55)
	b.Add(5)
	b.Add(95)

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge() returned error: %v", err)
	}

	want := []uint64{2, 0, 0, 0, 0, 1, 0, 0, 0, 1}
	if got := a.Counts(); !slices.Equal(got, want) {
		t.Errorf("Counts() = %v, want %v", got, want)
	}
}

func TestMergeSumsCounters(t *testing.T) {
	a := mustNew(t, 0, 100, 10)
	b := mustNew(t, 0, 100, 10)

	a.Add(-1)
	a.Add(150)
	a.Add(math.NaN())

	b.Add(-1)
	b.Add(-2)
	b.Add(200)
	b.Add(math.NaN())

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge() returned error: %v", err)
	}

	if got := a.Underflow(); got != 3 {
		t.Errorf("Underflow() = %d, want 3", got)
	}
	if got := a.Overflow(); got != 2 {
		t.Errorf("Overflow() = %d, want 2", got)
	}
	if got := a.Invalid(); got != 2 {
		t.Errorf("Invalid() = %d, want 2", got)
	}
}

func TestMergeDoesNotModifyOther(t *testing.T) {
	a := mustNew(t, 0, 100, 10)
	b := mustNew(t, 0, 100, 10)
	b.Add(5)
	b.Add(-1)

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge() returned error: %v", err)
	}

	if got := b.Counts()[0]; got != 1 {
		t.Errorf("other.Counts()[0] = %d, want 1", got)
	}
	if got := b.Underflow(); got != 1 {
		t.Errorf("other.Underflow() = %d, want 1", got)
	}
}

func TestMergeRejectsIncompatibleHistograms(t *testing.T) {
	tests := []struct {
		name    string
		lo, hi  float64
		numBins int
	}{
		{"different number of bins", 0, 100, 20},
		{"different lo", 10, 100, 10},
		{"different hi", 0, 200, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustNew(t, 0, 100, 10)
			a.Add(5)
			a.Add(-1)
			b := mustNew(t, tt.lo, tt.hi, tt.numBins)
			b.Add(50)

			if err := a.Merge(b); err == nil {
				t.Fatal("Merge() returned nil error, want an error")
			}

			want := make([]uint64, 10)
			want[0] = 1
			if got := a.Counts(); !slices.Equal(got, want) {
				t.Errorf("receiver modified by failed Merge: Counts() = %v, want %v", got, want)
			}
			if got := a.Underflow(); got != 1 {
				t.Errorf("receiver modified by failed Merge: Underflow() = %d, want 1", got)
			}
		})
	}
}

func TestMergeRejectsNil(t *testing.T) {
	h := mustNew(t, 0, 100, 10)

	if err := h.Merge(nil); err == nil {
		t.Error("Merge(nil) returned nil error, want an error")
	}
}

func randomValue(r *rand.Rand, lo, hi float64, numBins int) float64 {
	width := hi - lo

	switch r.IntN(10) {
	case 0:
		return math.NaN()
	case 1:
		return math.Inf(1)
	case 2:
		return math.Inf(-1)
	case 3:
		return lo // lower edge
	case 4:
		return hi // upper edge, counts as overflow
	case 5:
		k := r.IntN(numBins + 1)
		return lo + float64(k)*width/float64(numBins) // exact bin boundaries
	default:
		return lo - 0.2*width + r.Float64()*1.4*width // mostly in range, some under and over
	}
}

func assertSame(t *testing.T, got, want *Histogram) {
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

func TestMergeOfSplitDataEqualsAddingAll(t *testing.T) {
	const (
		lo      = 0.0
		hi      = 100.0
		numBins = 10
	)
	r := rand.New(rand.NewPCG(1, 2))

	for trial := 0; trial < 100; trial++ {
		numParts := 1 + r.IntN(8)
		all := mustNew(t, lo, hi, numBins)
		parts := make([]*Histogram, numParts)
		for i := range parts {
			parts[i] = mustNew(t, lo, hi, numBins)
		}

		for n := 0; n < 1000; n++ {
			v := randomValue(r, lo, hi, numBins)
			all.Add(v)
			parts[r.IntN(numParts)].Add(v)
		}

		merged := mustNew(t, lo, hi, numBins)
		for _, p := range parts {
			if err := merged.Merge(p); err != nil {
				t.Fatalf("trial %d: Merge() returned error: %v", trial, err)
			}
		}

		t.Logf("trial %d: %d parts", trial, numParts)
		assertSame(t, merged, all)
	}
}

func TestBinEdges(t *testing.T) {
	tests := []struct {
		name    string
		lo, hi  float64
		numBins int
		want    []float64
	}{
		{"ten bins", 0, 100, 10, []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}},
		{"negative lo", -1, 1, 4, []float64{-1, -0.5, 0, 0.5, 1}},
		{"single bin", 0, 5, 1, []float64{0, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mustNew(t, tt.lo, tt.hi, tt.numBins)

			if got := h.BinEdges(); !slices.Equal(got, tt.want) {
				t.Errorf("BinEdges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinEdgesEndpointsAreExact(t *testing.T) {
	for _, lo := range []float64{-1, 0, 0.1, 1, 1000} {
		for _, width := range []float64{0.3, 1, 7, 100} {
			hi := lo + width

			for numBins := 1; numBins <= 50; numBins++ {
				edges := mustNew(t, lo, hi, numBins).BinEdges()

				if len(edges) != numBins+1 {
					t.Fatalf("lo=%v hi=%v numBins=%d: len(edges) = %d, want %d", lo, hi, numBins, len(edges), numBins+1)
				}
				if edges[0] != lo || edges[numBins] != hi {
					t.Errorf("lo=%v hi=%v numBins=%d: edges ends at [%v, %v], want [%v, %v]",
						lo, hi, numBins, edges[0], edges[numBins], lo, hi)
				}
			}
		}
	}
}
